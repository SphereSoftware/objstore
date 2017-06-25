package objstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/xlab/objstore/cluster"
	"github.com/xlab/objstore/journal"
	"github.com/xlab/objstore/storage"
)

type Store interface {
	NodeID() string
	WaitOutbound(timeout time.Duration)
	WaitInbound(timeout time.Duration)
	ReceiveEventAnnounce(event *EventAnnounce)
	EmitEventAnnounce(event *EventAnnounce)
	DiskStats() (*DiskStats, error)
	Close() error

	// HeadObject gets object's meta data from the local journal.
	HeadObject(id string) (*FileMeta, error)
	// GetObject gets an object from the local storage of the node.
	// Used for private API, when other nodes ask for an object.
	GetObject(id string) (io.ReadCloser, *FileMeta, error)
	// FindObject gets and object from any node.
	// Used for public API, when file should be found somewhere.
	// Calls GetObject on all other nodes in parallel.
	FindObject(ctx context.Context, id string) (io.ReadCloser, *FileMeta, error)
	// FetchObject retrieves an object from remote storage, e.g. Amazon S3.
	// This should be called only on a total cache miss, when file is not found
	// on any node of the cluster.
	FetchObject(ctx context.Context, id string) (io.ReadCloser, *FileMeta, error)
}

var ErrNotFound = errors.New("not found")

type DiskStats storage.DiskStats

type EventAnnounce cluster.EventAnnounce

const (
	EventOpaqueData cluster.EventType = cluster.EventOpaqueData
)

type objStore struct {
	nodeID string

	localStorage  storage.LocalStorage
	remoteStorage storage.RemoteStorage
	journals      journal.JournalManager
	cluster       cluster.ClusterManager

	outboundWg        *sync.WaitGroup
	outboundPump      chan *EventAnnounce
	outboundAnnounces chan *EventAnnounce

	inboundWg        *sync.WaitGroup
	inboundPump      chan *EventAnnounce
	inboundAnnounces chan *EventAnnounce
}

func NewStore(nodeID string,
	localStorage storage.LocalStorage,
	remoteStorage storage.RemoteStorage,
	journals journal.JournalManager,
	cluster cluster.ClusterManager,
) (Store, error) {
	if !checkUUID(nodeID) {
		return nil, errors.New("objstore: invalid node ID")
	}
	if localStorage == nil {
		return nil, errors.New("objstore: local storage not provided")
	}
	if remoteStorage == nil {
		return nil, errors.New("objstore: remote storage not provided")
	}
	if journals == nil {
		return nil, errors.New("objstore: journals manager not provided")
	}
	if cluster == nil {
		return nil, errors.New("objstore: cluster manager not provided")
	}
	if err := localStorage.CheckAccess(""); err != nil {
		err = fmt.Errorf("objstore: cannot access local storage: %v", err)
		return nil, err
	}
	if err := remoteStorage.CheckAccess(""); err != nil {
		err = fmt.Errorf("objstore: cannot access remote storage: %v", err)
		return nil, err
	}
	if err := journals.Create(journal.ID(nodeID)); err != nil {
		err = fmt.Errorf("objstore: unable to create new journal: %v", err)
		return nil, err
	}
	outboundAnnounces := make(chan *EventAnnounce, 1024)
	inboundAnnounces := make(chan *EventAnnounce, 1024)
	store := &objStore{
		nodeID: nodeID,

		localStorage:  localStorage,
		remoteStorage: remoteStorage,
		journals:      journals,
		cluster:       cluster,

		outboundWg:        new(sync.WaitGroup),
		outboundPump:      pumpEventAnnounces(outboundAnnounces),
		outboundAnnounces: outboundAnnounces,

		inboundWg:        new(sync.WaitGroup),
		inboundPump:      pumpEventAnnounces(inboundAnnounces),
		inboundAnnounces: inboundAnnounces,
	}
	store.processInbound(4, 20*time.Second)
	store.processOutbound(4, 10*time.Minute)
	return store, nil
}

func (o *objStore) processOutbound(workers int, emitTimeout time.Duration) {
	for i := 0; i < workers; i++ {
		o.outboundWg.Add(1)
		go func() {
			defer o.outboundWg.Done()
			for ev := range o.outboundAnnounces {
				if err := o.emitEvent(ev, emitTimeout); err != nil {
					log.Println("[WARN] emitting event:", err)
				}
			}
		}()
	}
}

func (o *objStore) processInbound(workers int, connTimeout time.Duration) {
	for i := 0; i < workers; i++ {
		o.inboundWg.Add(1)
		go func() {
			defer o.inboundWg.Done()
			for ev := range o.inboundAnnounces {
				if err := o.handleEvent(ev, connTimeout); err != nil {
					log.Println("[WARN] handling event:", err)
				}
			}
		}()
	}
}

func (o *objStore) Close() error {
	o.inboundPump <- &EventAnnounce{
		Type: cluster.EventStopAnnounce,
	}
	o.outboundPump <- &EventAnnounce{
		Type: cluster.EventStopAnnounce,
	}
	return nil
}

func (o *objStore) WaitOutbound(timeout time.Duration) {
	waitWG(o.outboundWg, timeout)
}

func (o *objStore) WaitInbound(timeout time.Duration) {
	waitWG(o.inboundWg, timeout)
}

func waitWG(wg *sync.WaitGroup, timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		select {
		case <-done:
		default:
			close(done)
		}
	}()
	select {
	case <-time.Tick(timeout):
	case <-done:
	}
}

// ReceiveEventAnnounce never blocks. Internal workers will eventually handle the received events.
func (o *objStore) ReceiveEventAnnounce(event *EventAnnounce) {
	if event.Type == cluster.EventStopAnnounce {
		return
	}
	o.inboundPump <- event
}

// EmitEventAnnounce never blocks. Internal workers will eventually handle the events to emit.
func (o *objStore) EmitEventAnnounce(event *EventAnnounce) {
	if event.Type == cluster.EventStopAnnounce {
		return
	}
	o.outboundPump <- event
}

func (s *objStore) NodeID() string {
	return s.nodeID
}

func GenerateID() string {
	return journal.PseudoUUID()
}

func checkUUID(id string) bool {
	if len(id) == 0 {
		return false
	}
	// TODO: more checks
	return true
}

func (o *objStore) emitEvent(ev *EventAnnounce, timeout time.Duration) error {
	wg := new(sync.WaitGroup)
	defer wg.Wait()

	ctx, _ := context.WithTimeout(context.Background(), timeout)
	nodes, err := o.cluster.ListNodes()
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if node.ID == o.nodeID {
			continue
		}
		wg.Add(1)
		go func(node *cluster.NodeInfo) {
			defer wg.Done()
			if err := o.cluster.Announce(ctx, node.ID, (*cluster.EventAnnounce)(ev)); err != nil {
				log.Println("[WARN] announce error:", err)
			}
		}(node)
	}
	return nil
}

func (o *objStore) handleEvent(ev *EventAnnounce, timeout time.Duration) error {
	switch ev.Type {
	case cluster.EventFileAdded:
		// TODO
	case cluster.EventFileDeleted:

	case cluster.EventOpaqueData:
		log.Println("[INFO] cluster message:", string(ev.OpaqueData))
	default:
		log.Println("[WARN] skipping illegal cluster event type", ev.Type)
	}
	return nil
}

func (o *objStore) DiskStats() (*DiskStats, error) {
	ds, err := o.localStorage.DiskStats()
	if err != nil {
		return nil, err
	}
	return (*DiskStats)(ds), nil
}

type FileMeta journal.FileMeta

func (o *objStore) HeadObject(id string) (*FileMeta, error) {
	var meta *FileMeta
	err := o.journals.ForEach(func(j journal.Journal, _ *journal.JournalMeta) error {
		if m := j.Get(id); m == nil {
			return nil
		} else {
			meta = (*FileMeta)(m)
		}
		return journal.ForEachStop
	})
	return meta, err
}

func (o *objStore) FetchObject(ctx context.Context, id string) (io.ReadCloser, *FileMeta, error) {
	// spec, err := o.remoteStorage.GetObject(id)

	// TODO: map to ReadCloser & meta
	panic("not implemented")
}

func (o *objStore) GetObject(id string) (io.ReadCloser, *FileMeta, error) {
	var meta *FileMeta
	err := o.journals.ForEach(func(j journal.Journal, _ *journal.JournalMeta) error {
		if m := j.Get(id); m == nil {
			return nil
		} else {
			meta = (*FileMeta)(m)
		}
		return journal.ForEachStop
	})
	if err != nil {
		return nil, nil, err
	} else if meta == nil {
		return nil, nil, ErrNotFound
	}
	if meta.IsSymlink {
		// file should be located somewhere else, we don't have that file
		return nil, meta, ErrNotFound
	}
	f, err := o.localStorage.Read(id)
	if err != nil {
		log.Println("[WARN] file not found on disk:", (*journal.FileMeta)(meta).String())
		return nil, meta, ErrNotFound
	}
	return f, meta, nil
}

func (o *objStore) FindObject(ctx context.Context, id string) (io.ReadCloser, *FileMeta, error) {
	r, meta, err := o.GetObject(id)
	if err == nil {
		// found locally
		return r, meta, nil
	} else if err != ErrNotFound {
		log.Println("[WARN]", err)
	}
	if meta == nil {
		// completely not found -> file been removed
		return nil, nil, ErrNotFound
	}

	nodes, err := o.cluster.ListNodes()
	if err != nil {
		err = fmt.Errorf("objstore: cannot discover nodes: %v", err)
		return nil, nil, err
	} else if len(nodes) < 2 {
		// no other nodes except us..
		return nil, nil, ErrNotFound
	}
	found := make(chan io.ReadCloser, len(nodes))
	wg := new(sync.WaitGroup)
	for _, node := range nodes {
		if node.ID == o.nodeID {
			continue
		}
		wg.Add(1)
		go func(node *cluster.NodeInfo) {
			defer wg.Done()
			if r, err := o.cluster.GetObject(ctx, node.ID, id); err == nil {
				found <- r
			} else if err != cluster.ErrNotFound {
				log.Println("[WARN] cluster error:", err)
			}
		}(node)
	}

	go func() {
		wg.Wait()
		close(found)
	}()
	// found will be closed if all workers done,
	// or we get at least 1 result from the channel.
	if r, ok := <-found; ok {
		return r, meta, nil
	}
	return nil, nil, ErrNotFound
}

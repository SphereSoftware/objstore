package objstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/oklog/ulid"
	"github.com/xlab/closer"

	"sphere.software/objstore/cluster"
	"sphere.software/objstore/journal"
	"sphere.software/objstore/storage"
)

type Store interface {
	NodeID() string
	IsReady() bool
	SetDebug(v bool)
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
	// FindObject gets and object from any node, if not found then tries to acquire from
	// the remote storage, e.g. Amazon S3.
	FindObject(ctx context.Context, id string, fetch bool) (io.ReadCloser, *FileMeta, error)
	// FetchObject retrieves an object from the remote storage, e.g. Amazon S3.
	// This should be called only on a total cache miss, when file is not found
	// on any node of the cluster.
	FetchObject(ctx context.Context, id string) (io.ReadCloser, *FileMeta, error)
	// PutObject writes object to the local storage, emits cluster announcements, optionally
	// writes object to remote storage, e.g. Amazon S3. Returns amount of bytes written.
	PutObject(r io.ReadCloser, meta *FileMeta) (int64, error)
	// DeleteObject marks object as deleted in journals and deletes it from the local storage.
	// This operation does not delete object from remote storage.
	DeleteObject(id string) (*FileMeta, error)
	// Diff finds the difference between serialized exernal journal represented as list,
	// and journals currently available on this local node.
	Diff(list FileMetaList) (added, deleted FileMetaList, err error)
}

var ErrNotFound = errors.New("not found")

type DiskStats storage.DiskStats

type EventAnnounce cluster.EventAnnounce

type ConsistencyLevel journal.ConsistencyLevel

func (c ConsistencyLevel) Check() (journal.ConsistencyLevel, error) {
	level := (journal.ConsistencyLevel)(c)
	switch level {
	case journal.ConsistencyLocal, journal.ConsistencyS3, journal.ConsistencyFull:
		return level, nil
	default:
		return 0, errors.New("objstore: invalid consistency level")
	}
}

const (
	EventOpaqueData cluster.EventType = cluster.EventOpaqueData
)

type storeState int

const (
	storeInactiveState storeState = 0
	storeSyncState     storeState = 1
	storeActiveState   storeState = 2
)

type objStore struct {
	nodeID string
	debug  bool

	stateMux *sync.RWMutex
	state    storeState

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
	if !CheckID(nodeID) {
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
		nodeID:   nodeID,
		stateMux: new(sync.RWMutex),

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
	store.processInbound(4, 10*time.Minute)
	store.processOutbound(4, 10*time.Minute)
	go func() {
		time.Sleep(2 * time.Second)
		var synced bool
		for !synced {
			synced = store.sync(10 * time.Minute)
			time.Sleep(2 * time.Second)
		}
		if store.debug {
			log.Println("[INFO] sync done")
		}
	}()
	go func() {
		listJournals := func() {
			list, err := store.journals.ListAll()
			if err != nil {
				log.Println("[WARN] error listing journals", err)
				return
			}
			log.Println("[INFO] node journals:")
			log.Println(list)
		}
		for {
			for !store.IsReady() {
				time.Sleep(2 * time.Second)
			}
			if store.debug {
				listJournals()
			}
			ts := time.Now()
			_, err := store.journals.JoinAll(journal.ID(nodeID))
			if err != nil {
				log.Println("[WARN] journal consolidation failed:", err)
			} else if store.debug {
				log.Println("[INFO] consolidation done in", time.Since(ts))
				listJournals()
			}
			time.Sleep(24 * time.Hour)
		}
	}()
	return store, nil
}

func (o *objStore) sync(timeout time.Duration) bool {
	nodes, err := o.cluster.ListNodes()
	if err != nil {
		closer.Fatalln("[WARN] list nodes failed, sync cancelled:", err)
	} else if len(nodes) < 2 {
		o.stateMux.Lock()
		o.state = storeActiveState
		o.stateMux.Unlock()
		return false
	}
	o.stateMux.Lock()
	o.state = storeInactiveState
	o.stateMux.Unlock()

	list, err := o.journals.ExportAll()
	if err != nil {
		closer.Fatalln("[WARN] list journals failed, sync cancelled:", err)
	}

	wg := new(sync.WaitGroup)
	ctx, _ := context.WithTimeout(context.Background(), timeout)

	var listAdded journal.FileMetaList
	var listDeleted journal.FileMetaList

	for _, node := range nodes {
		if node.ID == o.nodeID {
			continue
		}
		wg.Add(1)
		go func(node *cluster.NodeInfo) {
			defer wg.Done()

			added, deleted, err := o.cluster.Sync(ctx, node.ID, list)
			if err != nil {
				log.Println("[WARN] sync error:", err)
			} else {
				listAdded = append(listAdded, added...)
				listDeleted = append(listDeleted, deleted...)
			}
		}(node)
	}
	wg.Wait()

	setAdded := make(map[string]*journal.FileMeta, len(listAdded))
	setDeleted := make(map[string]*journal.FileMeta, len(listDeleted))

	for _, meta := range listAdded {
		if m, ok := setAdded[meta.ID]; ok {
			if meta.Timestamp > m.Timestamp {
				setAdded[meta.ID] = meta
				continue
			}
		}
		setAdded[meta.ID] = meta
	}
	for _, meta := range listDeleted {
		if mAdd, ok := setAdded[meta.ID]; ok {
			// added already, check priority by age
			if mAdd.Timestamp > meta.Timestamp {
				continue // skip this delete event
			} else {
				delete(setAdded, meta.ID)
			}
		}
		if m, ok := setDeleted[meta.ID]; ok {
			if meta.Timestamp > m.Timestamp {
				setDeleted[meta.ID] = meta
				continue
			}
		}
		setDeleted[meta.ID] = meta
	}

	err = o.journals.Update(journal.ID(o.nodeID),
		func(j journal.Journal, _ *journal.JournalMeta) error {
			for _, meta := range setAdded {
				if meta.IsDeleted {
					// missing in our records, but marked as deleted elsewere
					o.localStorage.Delete(meta.ID)
					meta.IsSymlink = true
					if err := j.Set(meta.ID, meta); err != nil {
						log.Println("[WARN] journal set:", err)
					}
					continue
				}
				switch meta.Consistency {
				case journal.ConsistencyLocal, journal.ConsistencyS3:
					// stored elsewere
					meta.IsSymlink = true
					if err := j.Set(meta.ID, meta); err != nil {
						log.Println("[WARN] journal set:", err)
					}
				case journal.ConsistencyFull:
					// must replicate, i.e. handle the missing announce
					meta.IsSymlink = true // temporarily, will be overridden once replicated
					o.ReceiveEventAnnounce(&EventAnnounce{
						Type:     cluster.EventFileAdded,
						FileMeta: meta,
					})
					if err := j.Set(meta.ID, meta); err != nil {
						log.Println("[WARN] journal set:", err)
					}
				}
			}
			return nil
		})
	if err != nil {
		closer.Fatalln("[WARN] failed to sync journal:", err)
	}

	o.stateMux.Lock()
	o.state = storeActiveState
	o.stateMux.Unlock()

	for _, meta := range setDeleted {
		if meta.IsDeleted {
			// some nodes missing info we have on deleted object
			o.EmitEventAnnounce(&EventAnnounce{
				Type:     cluster.EventFileDeleted,
				FileMeta: meta,
			})
			continue
		}
		// some nodes are missing our file
		o.EmitEventAnnounce(&EventAnnounce{
			Type:     cluster.EventFileAdded,
			FileMeta: meta,
		})
	}

	return true
}

func (o *objStore) processOutbound(workers int, emitTimeout time.Duration) {
	for i := 0; i < workers; i++ {
		o.outboundWg.Add(1)
		go func() {
			defer o.outboundWg.Done()

			for !o.IsReady() {
				time.Sleep(100 * time.Millisecond)
			}
			for ev := range o.outboundAnnounces {
				if err := o.emitEvent(ev, emitTimeout); err != nil {
					log.Println("[WARN] emitting event:", err)
				}
			}
		}()
	}
}

func (o *objStore) processInbound(workers int, timeout time.Duration) {
	for i := 0; i < workers; i++ {
		o.inboundWg.Add(1)
		go func() {
			defer o.inboundWg.Done()

			for !o.IsReady() {
				time.Sleep(100 * time.Millisecond)
			}
			for ev := range o.inboundAnnounces {
				if err := o.handleEvent(ev, timeout); err != nil {
					log.Println("[WARN] handling event:", err)
				}
			}
		}()
	}
}

func (o *objStore) IsReady() bool {
	o.stateMux.RLock()
	ready := o.state == storeActiveState
	o.stateMux.RUnlock()
	return ready
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
	return journal.GetULID()
}

func CheckID(str string) bool {
	id, err := ulid.Parse(str)
	if err != nil {
		log.Println("[WARN] ULID is invalid: %s: %v", str, err)
		return false
	}
	ts := time.Unix(int64(id.Time()/1000), 0)
	if ts.Before(time.Date(2010, 0, 0, 0, 0, 0, 0, time.UTC)) ||
		ts.After(time.Date(2100, 0, 0, 0, 0, 0, 0, time.UTC)) {
		log.Println("[WARN] ULID has timestamp:", ts, "which is not current")
		return false
	}
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

func (o *objStore) findOnCluster(ctx context.Context, id string) (io.ReadCloser, error) {
	nodes, err := o.cluster.ListNodes()
	if err != nil {
		err = fmt.Errorf("objstore: cannot discover nodes: %v", err)
		return nil, err
	} else if len(nodes) < 2 {
		// no other nodes except us..
		return nil, ErrNotFound
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
		return r, nil
	}
	return nil, ErrNotFound
}

func (o *objStore) handleEvent(ev *EventAnnounce, timeout time.Duration) error {
	switch ev.Type {
	case cluster.EventFileAdded:
		if ev.FileMeta == nil {
			log.Println("[WARN] skipping added event with no meta")
			return nil
		}
		id := ev.FileMeta.ID
		meta := (*FileMeta)(ev.FileMeta)
		if meta.Consistency == journal.ConsistencyFull {
			// need to replicate the file locally
			ctx, _ := context.WithTimeout(context.Background(), timeout)
			r, err := o.findOnCluster(ctx, id)
			if err == ErrNotFound {
				if o.debug {
					log.Println("[INFO] file not found on cluster:", ev.FileMeta)
				}
				// object not found on cluster, fetch from remote store
				r, meta, err = o.FetchObject(ctx, id)
				if err == ErrNotFound {
					// we simply bail out if the file is expected with full consistency but not
					// found on the cluster and the remote storage.
					log.Println("[WARN] unable to find object for:", ev.FileMeta)
					return nil
				}
				meta.Consistency = journal.ConsistencyFull
			}
			meta.IsSymlink = false
			if _, err := o.storeLocal(r, meta); err != nil {
				r.Close()
				log.Println("[WARN] failed to fetch and store object:", err)
				return nil
			}
			r.Close()
		} else {
			meta.IsSymlink = true
		}
		if err := o.journals.ForEachUpdate(func(j journal.Journal, _ *journal.JournalMeta) error {
			if j.ID() == journal.ID(o.nodeID) {
				return j.Set(id, (*journal.FileMeta)(meta))
			}
			return j.Delete(id)
		}); err != nil {
			return err
		}
	case cluster.EventFileDeleted:
		if ev.FileMeta == nil {
			log.Println("[WARN] skipping deleted event with no meta")
			return nil
		}
		var found bool
		id := ev.FileMeta.ID
		err := o.journals.ForEachUpdate(func(j journal.Journal, _ *journal.JournalMeta) error {
			if m := j.Get(id); m != nil {
				found = true
				m.IsDeleted = true
				m.Timestamp = time.Now().UnixNano()
				if err := j.Set(id, m); err != nil {
					return err
				}
				return journal.ForEachStop
			}
			return nil
		})
		if err != nil {
			err = fmt.Errorf("objstore: journal update failed: %v", err)
			return err
		} else if found {
			if err := o.localStorage.Delete(id); err != nil {
				log.Println("[WARN] failed to delete local file:", err)
			}
		}
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
type FileMetaList journal.FileMetaList

func (o *objStore) HeadObject(id string) (*FileMeta, error) {
	var meta *FileMeta
	err := o.journals.ForEach(func(j journal.Journal, _ *journal.JournalMeta) error {
		if m := j.Get(id); m != nil {
			meta = (*FileMeta)(m)
			return journal.ForEachStop
		}
		return nil
	})
	if err != nil {
		return nil, err
	} else if meta == nil {
		return nil, ErrNotFound
	}
	return meta, nil
}

func (o *objStore) GetObject(id string) (io.ReadCloser, *FileMeta, error) {
	var meta *FileMeta
	err := o.journals.ForEach(func(j journal.Journal, _ *journal.JournalMeta) error {
		if m := j.Get(id); m != nil {
			meta = (*FileMeta)(m)
			return journal.ForEachStop
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	} else if meta == nil {
		return nil, nil, ErrNotFound
	}
	if meta.IsSymlink {
		// file should be located somewhere else, we don't have that file
		return nil, meta, ErrNotFound
	} else if meta.IsDeleted {
		return nil, meta, ErrNotFound
	}
	f, err := o.localStorage.Read(id)
	if err != nil {
		log.Println("[WARN] file not found on disk:", (*journal.FileMeta)(meta).String())
		return nil, meta, ErrNotFound
	}
	return f, meta, nil
}

func (o *objStore) FindObject(ctx context.Context,
	id string, fetch bool) (io.ReadCloser, *FileMeta, error) {
	r, meta, err := o.GetObject(id)
	if err == nil {
		// found locally
		return r, meta, nil
	} else if err != ErrNotFound {
		log.Println("[WARN]", err)
	}
	if meta == nil && !fetch {
		// completely not found -> file has been removed
		return nil, nil, ErrNotFound
	} else if meta != nil {
		r, err = o.findOnCluster(ctx, id)
		if err == nil {
			return r, meta, err
		} else if err != ErrNotFound {
			log.Println("[WARN] error when finding object:", err)
		}
		if o.debug {
			log.Println("[INFO] file not found on cluster:", id)
		}
	}
	//  fetch from remote store
	r, meta, err = o.FetchObject(ctx, id)
	if err == ErrNotFound {
		return nil, nil, ErrNotFound
	}
	// store it locally
	meta.IsSymlink = false
	if (meta.Consistency) == 0 {
		meta.Consistency = journal.ConsistencyS3
	}
	meta.Timestamp = time.Now().UnixNano()
	if _, err := o.storeLocal(r, meta); err != nil {
		r.Close()
		log.Println("[WARN] failed to fetch and store object:", err)
		return nil, meta, err
	}
	r.Close()
	// update journals
	if err := o.journals.ForEachUpdate(func(j journal.Journal, _ *journal.JournalMeta) error {
		if j.ID() == journal.ID(o.nodeID) {
			return j.Set(id, (*journal.FileMeta)(meta))
		}
		return j.Delete(id)
	}); err != nil {
		return nil, meta, err
	}
	o.EmitEventAnnounce(&EventAnnounce{
		Type:     cluster.EventFileAdded,
		FileMeta: (*journal.FileMeta)(meta),
	})
	// serve from local storage
	f, err := o.localStorage.Read(id)
	if err != nil {
		log.Println("[WARN] file not found on disk:", meta)
		return nil, meta, ErrNotFound
	}
	copyMeta := *meta
	copyMeta.IsFetched = true
	return f, &copyMeta, nil
}

func (o *objStore) FetchObject(ctx context.Context, id string) (io.ReadCloser, *FileMeta, error) {
	spec, err := o.remoteStorage.GetObject(id)
	if err != nil {
		return nil, nil, err
	}
	meta := new(journal.FileMeta)
	meta.Unmap(spec.Meta)
	meta.ID = id
	if spec.Size > 0 {
		meta.Size = spec.Size
	}
	return spec.Body, (*FileMeta)(meta), nil
}

func (o *objStore) storeLocal(r io.Reader, meta *FileMeta) (written int64, err error) {
	written, err = o.localStorage.Write(meta.ID, r)
	if err != nil {
		return
	}
	journalID := journal.ID(o.nodeID)
	var journalOk bool
	if err = o.journals.ForEachUpdate(
		func(j journal.Journal, _ *journal.JournalMeta) error {
			if journalID == j.ID() {
				journalOk = true
				return j.Set(meta.ID, (*journal.FileMeta)(meta))
			}
			return j.Delete(meta.ID)
		}); err != nil {
		return
	}
	if !journalOk {
		err = fmt.Errorf("objstore: journal not found: %v", journalID)
		return
	}
	return
}

func (o *objStore) PutObject(r io.ReadCloser, meta *FileMeta) (int64, error) {
	switch meta.Consistency {
	case journal.ConsistencyLocal:
		written, err := o.storeLocal(r, meta)
		if err != nil {
			r.Close()
			err = fmt.Errorf("objstore: local store failed: %v", err)
			return written, err
		}
		r.Close()
		o.EmitEventAnnounce(&EventAnnounce{
			Type:     cluster.EventFileAdded,
			FileMeta: (*journal.FileMeta)(meta),
		})
	case journal.ConsistencyS3, journal.ConsistencyFull:
		written, err := o.storeLocal(r, meta)
		if err != nil {
			r.Close()
			err = fmt.Errorf("objstore: local store failed: %v", err)
			return written, err
		}
		r.Close()
		o.EmitEventAnnounce(&EventAnnounce{
			Type:     cluster.EventFileAdded,
			FileMeta: (*journal.FileMeta)(meta),
		})
		// for optimal S3 uploads we should provide io.ReadSeeker,
		// this is why we store object as local file first, then upload to S3.
		f, err := o.localStorage.Read(meta.ID)
		if err != nil {
			err = fmt.Errorf("objstore: local store missing file: %v", err)
			return written, err
		}
		defer f.Close()

		if _, err = o.remoteStorage.PutObject(meta.ID, f, (*journal.FileMeta)(meta).Map()); err != nil {
			err = fmt.Errorf("objstore: remote store failed: %v", err)
			return written, err
		}
		return written, nil
	default:
		return 0, fmt.Errorf("objstore: unknown consistency %v", meta.Consistency)
	}
	return 0, nil
}

func (o *objStore) DeleteObject(id string) (*FileMeta, error) {
	var meta *FileMeta
	err := o.journals.ForEachUpdate(func(j journal.Journal, _ *journal.JournalMeta) error {
		if m := j.Get(id); m != nil {
			m.IsDeleted = true
			m.Timestamp = time.Now().UnixNano()
			if err := j.Set(id, m); err != nil {
				return err
			}
			meta = (*FileMeta)(m)
			return journal.ForEachStop
		}
		return nil
	})
	if err != nil {
		return nil, err
	} else if meta == nil {
		return nil, ErrNotFound
	}
	o.EmitEventAnnounce(&EventAnnounce{
		Type:     cluster.EventFileDeleted,
		FileMeta: (*journal.FileMeta)(meta),
	})
	if err := o.localStorage.Delete(id); err != nil {
		log.Println("[WARN] failed to delete local file:", err)
	}
	return meta, nil
}

func (o *objStore) Diff(list FileMetaList) (added, deleted FileMetaList, err error) {
	internal, err := o.journals.ExportAll()
	if err != nil {
		err := fmt.Errorf("objstore: failed to collect journals: %v", err)
		return nil, nil, err
	}
	internalJournal := journal.MakeJournal("", internal)
	externalJournal := journal.MakeJournal("", (journal.FileMetaList)(list))
	add, del := externalJournal.Diff(internalJournal)
	return (FileMetaList)(add), (FileMetaList)(del), nil
}

func (o *objStore) SetDebug(v bool) {
	o.debug = v
}

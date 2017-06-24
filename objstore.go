package objstore

import (
	"errors"
	"sync"
	"time"

	"github.com/xlab/objstore/cluster"
	"github.com/xlab/objstore/journal"
	"github.com/xlab/objstore/storage"
)

type Store interface {
	NodeID() string
	WaitOutbound(timeout time.Duration) int
	WaitInbound(timeout time.Duration) int
	ReceiveEventAnnounce(event *EventAnnounce)
	EmitEventAnnounce(event *EventAnnounce)

	// ops: put, get, etc...
}

type EventAnnounce cluster.EventAnnounce

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

	// TODO: check storage access permissions!

	store := &objStore{
		nodeID: nodeID,

		localStorage:  localStorage,
		remoteStorage: remoteStorage,
		journals:      journals,
		cluster:       cluster,
	}
	return store, nil
}

func (o *objStore) WaitOutbound(timeout time.Duration) int {
	// TODO: wait until all workers process outbound events or timeout
	panic("TODO")
}

func (o *objStore) WaitInbound(timeout time.Duration) int {
	// TODO: wait until all workers process inbound events or timeout
	panic("TODO")
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

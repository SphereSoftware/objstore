package cluster

import "github.com/xlab/objstore/journal"

type EventType int

const (
	EventUnknown      EventType = 0
	EventFileAdded    EventType = 1
	EventFileDeleted  EventType = 2
	EventStopAnnounce EventType = 999
)

type EventAnnounce struct {
	Type EventType `json:"type"`

	FileMeta *journal.FileMeta `json:"file_meta"`
}

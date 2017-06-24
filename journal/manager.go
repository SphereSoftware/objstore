package journal

import "github.com/boltdb/bolt"

type JournalManager interface {
	ListAll() ([]*JournalMeta, error)
	JoinAll() (*JournalMeta, error)

	Create(id ID) (*JournalMeta, error)
}

func NewJournalManager(db *bolt.DB) JournalManager {
	return &kvJournalManager{
		db: db,
	}
}

type kvJournalManager struct {
	db *bolt.DB
}

func (kv *kvJournalManager) Create(id ID) (*JournalMeta, error) {
	// TODO: set created date in mapping
	panic("not implemented")
	return nil, nil
}

func (kv *kvJournalManager) ListAll() ([]*JournalMeta, error) {
	panic("not implemented")
}

func (kv *kvJournalManager) JoinAll() (*JournalMeta, error) {
	panic("not implemented")
}

var (
	mappingBucket  = []byte("mapping")
	journalsBucket = []byte("journals")
)

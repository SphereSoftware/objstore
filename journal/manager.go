package journal

import (
	"errors"
	"fmt"
	"time"

	"github.com/boltdb/bolt"
)

type JournalManager interface {
	ListAll() ([]*JournalMeta, error)
	JoinAll() (*JournalMeta, error)

	Create(id ID) error
	ForEach(fn JournalIter) error
	Close() error
}

type JournalIter func(journal Journal, meta *JournalMeta) error

func NewJournalManager(db *bolt.DB) JournalManager {
	return &kvJournalManager{
		db: db,
	}
}

type kvJournalManager struct {
	db *bolt.DB
}

func (kv *kvJournalManager) Create(id ID) error {
	return kv.db.Update(func(tx *bolt.Tx) error {
		journalID := []byte(id)
		mapping, err := tx.CreateBucketIfNotExists(mappingBucket)
		if err != nil {
			return err
		}
		journals, err := tx.CreateBucketIfNotExists(journalsBucket)
		if err != nil {
			return err
		}
		if mapping.Get(journalID) != nil {
			return errors.New("kvJournal: journal mapping exists")
		}
		if journals.Get(journalID) != nil {
			return errors.New("kvJournal: journal exists")
		}
		if _, err := journals.CreateBucket(journalID); err != nil {
			err = fmt.Errorf("kvJournal: failed to create journal bucket: %v", err)
			return err
		}
		meta := &JournalMeta{
			ID:        id,
			CreatedAt: time.Now().Unix(),
		}
		data, _ := meta.MarshalMsg(nil)
		return mapping.Put(journalID, data)
	})
}

func (kv *kvJournalManager) ListAll() ([]*JournalMeta, error) {
	var metas []*JournalMeta
	err := kv.db.View(func(tx *bolt.Tx) error {
		mapping := tx.Bucket(mappingBucket)
		metas = make([]*JournalMeta, 0, mapping.Stats().KeyN)
		cur := mapping.Cursor()

		id, data := cur.First()
		for id != nil {
			var meta *JournalMeta
			if data != nil {
				meta = new(JournalMeta)
				meta.UnmarshalMsg(data)
				metas = append(metas, meta)
			}
			id, data = cur.Next()
		}
		return nil
	})
	return metas, err
}

func (kv *kvJournalManager) JoinAll() (*JournalMeta, error) {
	// a b c -> a c -> c
	panic("not implemented")
}

func (kv *kvJournalManager) ForEach(fn JournalIter) error {
	return kv.db.View(func(tx *bolt.Tx) error {
		mapping := tx.Bucket(mappingBucket)
		journals := tx.Bucket(journalsBucket)
		cur := journals.Cursor()

		id, _ := cur.First()
		for id != nil {
			var meta *JournalMeta
			if data := mapping.Get(id); data != nil {
				meta = new(JournalMeta)
				meta.UnmarshalMsg(data)
			}

			// log.Println("view journal", string(id), "\n", meta)

			journal := NewJournal(ID(id), tx, journals.Bucket(id))
			if err := fn(journal, meta); err == RangeStop {
				return nil
			} else if err != nil {
				return err
			}
			id, _ = cur.Next()
		}
		return nil
	})
}

func (kv *kvJournalManager) Close() error {
	return kv.db.Close()
}

var (
	RangeStop   = errors.New("stop")
	ForEachStop = RangeStop
)

var (
	mappingBucket  = []byte("mapping")
	journalsBucket = []byte("journals")
)

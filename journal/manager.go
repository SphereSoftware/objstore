package journal

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/boltdb/bolt"
)

type JournalManager interface {
	Create(id ID) error
	View(id ID, fn JournalIter) error
	Update(id ID, fn JournalIter) error

	ForEach(fn JournalIter) error
	ForEachUpdate(fn JournalIter) error

	JoinAll(target ID) (*JournalMeta, error)
	ListAll() ([]*JournalMeta, error)
	ExportAll() (FileMetaList, error)

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
			CreatedAt: time.Now().UnixNano(),
		}
		data, _ := meta.MarshalMsg(nil)
		return mapping.Put(journalID, data)
	})
}

func (kv *kvJournalManager) View(id ID, fn JournalIter) error {
	return kv.db.View(func(tx *bolt.Tx) error {
		journalID := []byte(id)
		mapping := tx.Bucket(mappingBucket)
		journals := tx.Bucket(journalsBucket)
		data := mapping.Get(journalID)
		var meta *JournalMeta
		if data == nil {
			return errors.New("kvJournal: journal mapping not exists")
		} else {
			meta = new(JournalMeta)
			meta.UnmarshalMsg(data)
		}
		b := journals.Bucket(journalID)
		if b == nil {
			return errors.New("kvJournal: journal not exists")
		}
		journal := NewJournal(id, tx, b)
		return fn(journal, meta)
	})
}

func (kv *kvJournalManager) Update(id ID, fn JournalIter) error {
	return kv.db.Update(func(tx *bolt.Tx) error {
		journalID := []byte(id)
		mapping := tx.Bucket(mappingBucket)
		journals := tx.Bucket(journalsBucket)
		data := mapping.Get(journalID)
		var meta *JournalMeta
		if data == nil {
			return errors.New("kvJournal: journal mapping not exists")
		} else {
			meta = new(JournalMeta)
			meta.UnmarshalMsg(data)
		}
		b := journals.Bucket(journalID)
		if b == nil {
			return errors.New("kvJournal: journal not exists")
		}
		journal := NewJournal(id, tx, b)
		return fn(journal, meta)
	})
}

func (kv *kvJournalManager) ListAll() (metaList []*JournalMeta, err error) {
	err = kv.db.View(func(tx *bolt.Tx) error {
		mapping := tx.Bucket(mappingBucket)
		journals := tx.Bucket(journalsBucket)
		metaList = make([]*JournalMeta, 0, journals.Stats().KeyN)
		cur := journals.Cursor()

		id, _ := cur.First()
		for id != nil {
			journal := NewJournal(ID(id), tx, journals.Bucket(id))
			meta := journal.Meta()
			if data := mapping.Get(id); data != nil {
				extraMeta := new(JournalMeta)
				extraMeta.UnmarshalMsg(data)
				meta.CreatedAt = extraMeta.CreatedAt
				meta.JoinedAt = extraMeta.JoinedAt
			}
			metaList = append(metaList, meta)
			id, _ = cur.Next()
		}
		return nil
	})
	return
}

func (kv *kvJournalManager) JoinAll(target ID) (*JournalMeta, error) {
	kv.Create(target) // for safety reasons ensure that journal exists

	var targetMeta *JournalMeta
	err := kv.db.Update(func(tx *bolt.Tx) error {
		mapping := tx.Bucket(mappingBucket)
		journals := tx.Bucket(journalsBucket)
		cur := journals.Cursor()

		journalID := []byte(target)
		targetJournal := NewJournal(target, tx, journals.Bucket(journalID))

		id, _ := cur.First()
		for id != nil {
			if bytes.Equal(id, journalID) {
				id, _ = cur.Next()
				continue
			}
			journal := NewJournal(ID(id), tx, journals.Bucket(id))
			if _, err := journal.Range("", 0, func(k string, v *FileMeta) error {
				if targetJournal.Exists(k) {
					// disallow override upon consolidation from older journals
					return nil
				}
				return targetJournal.Set(k, v)
			}); err != nil {
				return err
			}

			meta := journal.Meta()
			meta.JoinedAt = time.Now().UnixNano()
			meta.ID = target // relocated journal
			if data := mapping.Get(id); data != nil {
				extraMeta := new(JournalMeta)
				extraMeta.UnmarshalMsg(data)
				meta.CreatedAt = extraMeta.CreatedAt
			}
			data, _ := meta.MarshalMsg(nil)
			if err := mapping.Put(id, data); err != nil {
				return err
			}
			if err := journals.DeleteBucket(id); err != nil {
				return err
			}
			id, _ = cur.Next()
		}

		targetMeta = targetJournal.Meta()
		if data := mapping.Get(journalID); data != nil {
			extraMeta := new(JournalMeta)
			extraMeta.UnmarshalMsg(data)
			targetMeta.CreatedAt = extraMeta.CreatedAt
		}
		data, _ := targetMeta.MarshalMsg(nil)
		return mapping.Put(journalID, data)
	})
	return targetMeta, err
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

func (kv *kvJournalManager) ForEachUpdate(fn JournalIter) error {
	return kv.db.Update(func(tx *bolt.Tx) error {
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

func (kv *kvJournalManager) ExportAll() (FileMetaList, error) {
	var list FileMetaList
	err := kv.db.View(func(tx *bolt.Tx) error {
		journals := tx.Bucket(journalsBucket)
		cur := journals.Cursor()
		id, _ := cur.First()
		for id != nil {
			journal := NewJournal(ID(id), tx, journals.Bucket(id))
			list = append(list, journal.List()...)
			id, _ = cur.Next()
		}
		return nil
	})
	return list, err
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

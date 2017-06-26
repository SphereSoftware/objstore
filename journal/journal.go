// Package journal is responsible for maintaining the inner state of the OBJSTORE,
// journals represent managed event logs that can be diffed, joined and stored as
// in-memory B-tree or in a BoltDB bucket. All operations on BoltDB are performed
// in the context of a transaction, so journals are ACID-compatible.
package journal

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/cznic/b"
)

type Journal interface {
	ID() ID
	Get(k string) *FileMeta
	Exists(k string) bool
	Set(k string, m *FileMeta) error
	Delete(k string) error
	Diff(j Journal) (added FileMetaList, deleted FileMetaList)
	Range(start string, limit int, fn func(k string, v *FileMeta) error) (string, error)
	Join(target Journal, mapping Mapping) error
	List() FileMetaList
	Close() error
	Meta() *JournalMeta
}

type kvJournal struct {
	id ID

	b  *bolt.Bucket
	tx *bolt.Tx
}

type btreeJournal struct {
	id ID

	t      *b.Tree
	mux    *sync.Mutex
	closed bool
}

func (b *btreeJournal) Close() error {
	b.mux.Lock()
	defer b.mux.Unlock()
	b.t.Close()
	b.closed = true
	return nil
}

func (j *kvJournal) Close() error {
	// no-op as kvJournal is managed by BoltDB transaction
	return nil
}

func (b *btreeJournal) Get(k string) *FileMeta {
	b.mux.Lock()
	if b.closed {
		b.mux.Unlock()
		return nil
	}
	v, ok := b.t.Get(k)
	b.mux.Unlock()
	if ok {
		return v.(*FileMeta)
	}
	return nil
}

func (b *btreeJournal) Exists(k string) bool {
	b.mux.Lock()
	if b.closed {
		b.mux.Unlock()
		return false
	}
	_, ok := b.t.Get(k)
	b.mux.Unlock()
	return ok
}

func (b *btreeJournal) Set(k string, m *FileMeta) error {
	if m == nil {
		return errors.New("journal: nil entries not allowed")
	}
	if len(k) == 0 {
		return errors.New("journal: zero-length keys not allowed")
	}
	b.mux.Lock()
	if b.closed {
		b.mux.Unlock()
		return closedErr
	}
	b.t.Set(k, m)
	b.mux.Unlock()
	return nil
}

func (b *btreeJournal) Delete(k string) error {
	if len(k) == 0 {
		return errors.New("journal: zero-length keys not allowed")
	}
	b.mux.Lock()
	if b.closed {
		b.mux.Unlock()
		return closedErr
	}
	b.t.Delete(k)
	b.mux.Unlock()
	return nil
}

var closedErr = errors.New("journal: closed already")

func (b *btreeJournal) Range(start string, limit int, fn func(k string, v *FileMeta) error) (string, error) {
	b.mux.Lock()
	if b.closed {
		b.mux.Unlock()
		return "", closedErr
	}
	iter, ok := b.t.Seek(start)
	b.mux.Unlock()
	if !ok {
		return "", nil
	}
	defer iter.Close()

	var processed int
	var lastK string
	for {
		b.mux.Lock()
		if b.closed {
			b.mux.Unlock()
			return "", closedErr
		}
		k, v, err := iter.Next()
		b.mux.Unlock()
		if err == nil {
			lastK = k.(string)
			if err := fn(k.(string), v.(*FileMeta)); err == ErrRangeStop {
				return lastK, nil
			} else if err != nil {
				return lastK, err
			}
		} else {
			return "", nil
		}
		processed++
		if limit > 0 && processed >= limit {
			break
		}
	}
	return lastK, nil
}

func (b *btreeJournal) Join(target Journal, mapping Mapping) error {
	return errors.New("journal: unjoinable journals")
}

func (b *btreeJournal) Meta() *JournalMeta {
	firstKey, _ := b.t.First()
	lastKey, _ := b.t.Last()
	return &JournalMeta{
		ID: b.id,

		FirstKey:   firstKey.(string),
		LastKey:    lastKey.(string),
		CountTotal: b.t.Len(),
	}
}

func (b *btreeJournal) ID() ID {
	return b.id
}

func (b *btreeJournal) List() FileMetaList {
	b.mux.Lock()
	if b.closed {
		b.mux.Unlock()
		return nil
	}
	iter, err := b.t.SeekFirst()
	b.mux.Unlock()
	if err == nil {
		defer iter.Close()
	}

	var list FileMetaList
	var v interface{}
	for err == nil {
		b.mux.Lock()
		_, v, err = iter.Next()
		b.mux.Unlock()
		if err == nil {
			list = append(list, v.(*FileMeta))
		}
	}
	return list
}

func (prev *btreeJournal) Diff(next Journal) (added FileMetaList, deleted FileMetaList) {
	switch next := next.(type) {
	case *btreeJournal:
		prev.mux.Lock()
		if prev.closed {
			prev.mux.Unlock()
			return nil, nil
		}
		prevIter, prevErr := prev.t.SeekFirst()
		prev.mux.Unlock()
		if prevErr == nil {
			defer prevIter.Close()
		}
		next.mux.Lock()
		if next.closed {
			next.mux.Unlock()
			return nil, nil
		}
		nextIter, nextErr := next.t.SeekFirst()
		next.mux.Unlock()
		if nextErr == nil {
			defer nextIter.Close()
		}

		switch {
		case prevErr == io.EOF && nextErr == io.EOF:
			return nil, nil
		case prevErr == io.EOF:
			// all added
			return next.List(), nil
		case nextErr == io.EOF:
			// all deleted
			return nil, prev.List()
		default:
			prev.mux.Lock()
			prevK, prevV, prevErr := prevIter.Next()
			prev.mux.Unlock()
			next.mux.Lock()
			nextK, nextV, nextErr := nextIter.Next()
			next.mux.Unlock()

			for {
				switch {
				case prevErr == io.EOF:
					if nextErr == io.EOF {
						// done
						return
					}
					added = append(added, nextV.(*FileMeta))
					// move next iterator
					next.mux.Lock()
					nextK, nextV, nextErr = nextIter.Next()
					next.mux.Unlock()
				case nextErr == io.EOF:
					if prevErr == io.EOF {
						// done
						return
					}
					deleted = append(deleted, prevV.(*FileMeta))
					// move prev iterator
					prev.mux.Lock()
					prevK, prevV, prevErr = prevIter.Next()
					prev.mux.Unlock()
				default:
					prevCmp := strings.Compare(prevK.(string), nextK.(string))
					switch {
					case prevCmp < 0: // nextK > prevK
						// prevK has been deleted
						deleted = append(deleted, prevV.(*FileMeta))
						// advance prev iter
						prev.mux.Lock()
						prevK, prevV, prevErr = prevIter.Next()
						prev.mux.Unlock()
					case prevCmp > 0: // nextK < prevK
						// nextK has been insterted
						added = append(added, nextV.(*FileMeta))
						// advance next iter
						next.mux.Lock()
						nextK, nextV, nextErr = nextIter.Next()
						next.mux.Unlock()
					default:
						// same key -> advance iterators
						prev.mux.Lock()
						prevK, prevV, prevErr = prevIter.Next()
						prev.mux.Unlock()
						next.mux.Lock()
						nextK, nextV, nextErr = nextIter.Next()
						next.mux.Unlock()
					}
				}
			}
		}
	case *kvJournal:
		prev.mux.Lock()
		if prev.closed {
			prev.mux.Unlock()
			return nil, nil
		}
		prevIter, prevErr := prev.t.SeekFirst()
		prev.mux.Unlock()
		if prevErr == nil {
			defer prevIter.Close()
		}
		nextIter := next.b.Cursor()

		switch {
		case prevErr == io.EOF && nextIter == nil:
			return nil, nil
		case prevErr == io.EOF:
			// all added
			return next.List(), nil
		case nextIter == nil:
			// all deleted
			return nil, prev.List()
		default:
			prev.mux.Lock()
			prevK, prevV, prevErr := prevIter.Next()
			prev.mux.Unlock()
			nextK, nextV := nextIter.Next()

			for {
				switch {
				case prevErr == io.EOF:
					if nextK == nil {
						// done
						return
					}
					if nextV != nil {
						meta := new(FileMeta)
						meta.UnmarshalMsg(nextV)
						added = append(added, meta)
					}
					// move next iterator
					nextK, nextV = nextIter.Next()
				case nextK == nil:
					if prevErr == io.EOF {
						// done
						return
					}
					deleted = append(deleted, prevV.(*FileMeta))
					// move prev iterator
					prev.mux.Lock()
					prevK, prevV, prevErr = prevIter.Next()
					prev.mux.Unlock()
				default:
					prevCmp := strings.Compare(prevK.(string), string(nextK))
					switch {
					case prevCmp < 0: // nextK > prevK
						// prevK has been deleted
						deleted = append(deleted, prevV.(*FileMeta))
						// advance prev iter
						prev.mux.Lock()
						prevK, prevV, prevErr = prevIter.Next()
						prev.mux.Unlock()
					case prevCmp > 0: // nextK < prevK
						// nextK has been insterted
						if nextV != nil {
							meta := new(FileMeta)
							meta.UnmarshalMsg(nextV)
							added = append(added, meta)
						}
						// advance next iter
						nextK, nextV = nextIter.Next()
					default:
						// same key -> advance iterators
						prev.mux.Lock()
						prevK, prevV, prevErr = prevIter.Next()
						prev.mux.Unlock()
						nextK, nextV = nextIter.Next()
					}
				}
			}
		}
	default:
		panic("indifferentiable types")
	}
}

func (prev *kvJournal) Diff(next Journal) (added FileMetaList, deleted FileMetaList) {
	switch next := next.(type) {
	case *kvJournal:
		prevIter := prev.b.Cursor()
		nextIter := next.b.Cursor()

		switch {
		case prevIter == nil && nextIter == nil:
			return nil, nil
		case prevIter == nil:
			// all added
			return next.List(), nil
		case nextIter == nil:
			// all deleted
			return nil, prev.List()
		default:
			prevK, prevV := prevIter.Next()
			nextK, nextV := nextIter.Next()

			for {
				switch {
				case prevK == nil:
					if nextK == nil {
						// done
						return
					}
					if nextV != nil {
						meta := new(FileMeta)
						meta.UnmarshalMsg(nextV)
						added = append(added, meta)
					}
					// move next iterator
					nextK, nextV = nextIter.Next()
				case nextK == nil:
					if prevK == nil {
						// done
						return
					}
					if prevV != nil {
						meta := new(FileMeta)
						meta.UnmarshalMsg(prevV)
						deleted = append(deleted, meta)
					}
					// move prev iterator
					prevK, prevV = prevIter.Next()
				default:
					prevCmp := strings.Compare(string(prevK), string(nextK))
					switch {
					case prevCmp < 0: // nextK > prevK
						// prevK has been deleted
						if prevV != nil {
							meta := new(FileMeta)
							meta.UnmarshalMsg(prevV)
							deleted = append(deleted, meta)
						}
						// advance prev iter
						prevK, prevV = prevIter.Next()
					case prevCmp > 0: // nextK < prevK
						// nextK has been insterted
						if nextV != nil {
							meta := new(FileMeta)
							meta.UnmarshalMsg(nextV)
							added = append(added, meta)
						}
						// advance next iter
						nextK, nextV = nextIter.Next()
					default:
						// same key -> advance iterators
						prevK, prevV = prevIter.Next()
						nextK, nextV = nextIter.Next()
					}
				}
			}
		}
	case *btreeJournal:
		next.mux.Lock()
		if next.closed {
			next.mux.Unlock()
			return nil, nil
		}
		nextIter, nextErr := next.t.SeekFirst()
		next.mux.Unlock()
		if nextErr == nil {
			defer nextIter.Close()
		}
		prevIter := prev.b.Cursor()

		switch {
		case nextErr == io.EOF && prevIter == nil:
			return nil, nil
		case nextErr == io.EOF:
			// all added
			return prev.List(), nil
		case prevIter == nil:
			// all deleted
			return nil, next.List()
		default:
			next.mux.Lock()
			nextK, nextV, nextErr := nextIter.Next()
			next.mux.Unlock()
			prevK, prevV := prevIter.Next()

			for {
				switch {
				case nextErr == io.EOF:
					if prevK == nil {
						// done
						return
					}
					if prevV != nil {
						meta := new(FileMeta)
						meta.UnmarshalMsg(prevV)
						added = append(added, meta)
					}
					// move prev iterator
					prevK, prevV = prevIter.Next()
				case prevK == nil:
					if nextErr == io.EOF {
						// done
						return
					}
					deleted = append(deleted, nextV.(*FileMeta))
					// move next iterator
					next.mux.Lock()
					nextK, nextV, nextErr = nextIter.Next()
					next.mux.Unlock()
				default:
					nextCmp := strings.Compare(nextK.(string), string(prevK))
					switch {
					case nextCmp < 0: // prevK > nextK
						// nextK has been deleted
						deleted = append(deleted, nextV.(*FileMeta))
						// advance next iter
						next.mux.Lock()
						nextK, nextV, nextErr = nextIter.Next()
						next.mux.Unlock()
					case nextCmp > 0: // prevK < nextK
						// prevK has been insterted
						if prevV != nil {
							meta := new(FileMeta)
							meta.UnmarshalMsg(prevV)
							added = append(added, meta)
						}
						// advance prev iter
						prevK, prevV = prevIter.Next()
					default:
						// same key -> advance iterators
						next.mux.Lock()
						nextK, nextV, nextErr = nextIter.Next()
						next.mux.Unlock()
						prevK, prevV = prevIter.Next()
					}
				}
			}
		}
	default:
		panic("journal: indifferentiable types")
	}
}

// Join appends the current journal to the target one, reassigning atomically the mapping.
func (j *kvJournal) Join(target Journal, mapping Mapping) error {
	kvTarget, ok := target.(*kvJournal)
	if !ok {
		return errors.New("journal: unjoinable journals")
	}
	meta := mapping.Get(j.id)
	if meta == nil {
		// somehow mapping not available in the current Tx
		meta = j.Meta()
	} else if meta.ID != j.id {
		// ID mismatch -> already joined?
		err := fmt.Errorf("journal: already joined %s -> %s", j.id, meta.ID)
		return err
	}
	cur := j.b.Cursor()
	k, v := cur.First()
	var copied int
	for k != nil {
		if v == nil {
			continue
		}
		copied++
		if err := kvTarget.b.Put(k, v); err != nil {
			return err
		}
		k, v = cur.Next()
	}

	meta.JoinedAt = time.Now().UnixNano()
	meta.ID = target.ID() // relocate mapping
	mapping.Set(j.id, meta)
	return nil
}

func (j *kvJournal) Range(start string, limit int, fn func(k string, v *FileMeta) error) (string, error) {
	cur := j.b.Cursor()
	k, v := cur.Seek([]byte(start))
	var processed int
	for k != nil {
		var meta *FileMeta
		if v != nil {
			meta = new(FileMeta)
			meta.UnmarshalMsg(v)
		}
		if err := fn(string(k), meta); err == ErrRangeStop {
			return string(k), nil
		} else if err != nil {
			return string(k), err
		}
		k, v = cur.Next()
		processed++
		if limit > 0 && processed >= limit {
			return string(k), nil
		}
	}
	return "", nil
}

func (j *kvJournal) Get(k string) *FileMeta {
	data := j.b.Get([]byte(k))
	if data == nil {
		return nil
	}
	meta := new(FileMeta)
	meta.UnmarshalMsg(data)
	return meta
}

func (j *kvJournal) Exists(k string) bool {
	return j.b.Get([]byte(k)) != nil
}

func (j *kvJournal) Set(k string, m *FileMeta) error {
	v, err := m.MarshalMsg(nil)
	if err != nil {
		return err
	}
	return j.b.Put([]byte(k), v)
}

func (j *kvJournal) Delete(k string) error {
	return j.b.Delete([]byte(k))
}

func (j *kvJournal) List() FileMetaList {
	cur := j.b.Cursor()
	k, v := cur.First()
	var list FileMetaList
	for k != nil {
		if v != nil {
			meta := new(FileMeta)
			meta.UnmarshalMsg(v)
			list = append(list, meta)
		}
		k, v = cur.Next()
	}
	return list
}

func (j *kvJournal) ID() ID {
	return j.id
}

func (j *kvJournal) Meta() *JournalMeta {
	cur := j.b.Cursor()
	firstKey, _ := cur.First()
	lastKey, _ := cur.Last()
	return &JournalMeta{
		ID:         j.id,
		FirstKey:   string(firstKey),
		LastKey:    string(lastKey),
		CountTotal: j.b.Stats().KeyN,
	}
}

var ErrRangeStop = errors.New("range stop")

// NewJournal creates a new journal backed by a BoltDB bucket,
// in the context of a transaction.
func NewJournal(id ID, tx *bolt.Tx, bucket *bolt.Bucket) Journal {
	return &kvJournal{
		id: id,
		tx: tx,
		b:  bucket,
	}
}

// MakeJournal allows to represent a serialized list of events
// as an in-memory journal compatible with journals backed by a real KV store.
func MakeJournal(id ID, events FileMetaList) Journal {
	j := &btreeJournal{
		id:  id,
		mux: new(sync.Mutex),
		t: b.TreeNew(func(a interface{}, b interface{}) int {
			return strings.Compare(a.(string), b.(string))
		}),
	}
	for i := range events {
		j.t.Set(string(events[i].ID), events[i])
	}
	return j
}

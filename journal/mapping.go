package journal

import "github.com/boltdb/bolt"

type Mapping interface {
	Get(id ID) *JournalMeta
	Set(id ID, meta *JournalMeta) error
	SetBytes(k, v []byte) error
}

type mapping struct {
	tx *bolt.Tx
	b  *bolt.Bucket
}

func NewMapping(tx *bolt.Tx) (Mapping, error) {
	b, err := tx.CreateBucketIfNotExists(mappingBucket)
	if err != nil {
		return nil, err
	}
	m := &mapping{
		tx: tx,
		b:  b,
	}
	return m, nil
}

func (m *mapping) Get(id ID) *JournalMeta {
	data := m.b.Get([]byte(id))
	if data == nil {
		return nil
	}
	meta := new(JournalMeta)
	meta.UnmarshalMsg(data)
	return meta
}

func (m *mapping) Set(id ID, meta *JournalMeta) error {
	v, err := meta.MarshalMsg(nil)
	if err != nil {
		return err
	}
	return m.b.Put([]byte(id), v)
}

func (m *mapping) SetBytes(k, v []byte) error {
	return m.b.Put(k, v)
}

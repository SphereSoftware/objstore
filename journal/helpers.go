package journal

import (
	"math/rand"
	"sync"
	"time"

	"github.com/oklog/ulid"
)

var globalRand = rand.New(&lockedSource{
	src: rand.NewSource(time.Now().UnixNano()),
})

// GetULID constucts an Universally Unique Lexicographically Sortable Identifier.
// See https://github.com/oklog/ulid
func GetULID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), globalRand).String()
}

type lockedSource struct {
	lk  sync.Mutex
	src rand.Source
}

func (r *lockedSource) Int63() (n int64) {
	r.lk.Lock()
	n = r.src.Int63()
	r.lk.Unlock()
	return
}

func (r *lockedSource) Seed(seed int64) {
	r.lk.Lock()
	r.src.Seed(seed)
	r.lk.Unlock()
}

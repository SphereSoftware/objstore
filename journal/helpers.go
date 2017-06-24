package journal

import (
	"encoding/hex"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
)

func trimPad(v string) string {
	switch {
	case len(v) == 0:
		return ""
	case len(v) < 6:
		return strings.Repeat("0", 6-len(v)) + v
	default:
		return v[:6]
	}
}

var globalRand = rand.New(&lockedSource{
	src: rand.NewSource(time.Now().UnixNano()),
})

// 9 bytes of unixnano and 7 random bytes
func PseudoUUID() string {
	now := time.Now().UnixNano()
	randPart := make([]byte, 7)
	if _, err := globalRand.Read(randPart); err != nil {
		copy(randPart, (*(*[8]byte)(unsafe.Pointer(&now)))[:7])
	}
	return strconv.FormatInt(now, 10)[1:] + hex.EncodeToString(randPart)
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

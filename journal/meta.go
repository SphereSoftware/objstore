package journal

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type FileMeta struct {
	ID          string            `msgp:"0" json:"id"`
	Name        string            `msgp:"1" json:"name"`
	Size        int64             `msgp:"2" json:"size"`
	Timestamp   int64             `msgp:"3" json:"timestamp"`
	UserMeta    map[string]string `msgp:"4" json:"user_meta"`
	IsSymlink   bool              `msgp:"5" json:"is_symlink"`
	Consistency ConsistencyLevel  `msgp:"6" json:"consistency"`
	IsDeleted   bool              `msgp:"7" json:"is_deleted"`
	IsFetched   bool              `msgp:"8" json:"is_fetched"`
}

func (f *FileMeta) Map() map[string]string {
	m := map[string]string{
		"id":          f.ID,
		"name":        f.Name,
		"size":        strconv.FormatInt(f.Size, 10),
		"timestamp":   strconv.FormatInt(f.Timestamp, 10),
		"consistency": strconv.Itoa(int(f.Consistency)),
	}
	for k, v := range f.UserMeta {
		m["usermeta-"+k] = v
	}
	return m
}

func (f *FileMeta) Unmap(m map[string]string) {
	userMeta := make(map[string]string, len(m))
	for k, v := range m {
		switch k = strings.ToLower(k); k {
		case "id":
			f.ID = v
		case "name":
			f.Name = v
		case "size":
			f.Size, _ = strconv.ParseInt(v, 10, 64)
		case "timestamp":
			f.Timestamp, _ = strconv.ParseInt(v, 10, 64)
		case "consistency":
			level, _ := strconv.Atoi(v)
			if level == 0 {
				// at least
				f.Consistency = ConsistencyS3
			} else {
				f.Consistency = (ConsistencyLevel)(level)
			}
		default:
			if !strings.HasPrefix(k, "usermeta-") {
				continue
			}
			k = strings.TrimPrefix(k, "usermeta-")
			userMeta[k] = v
		}
	}
	f.UserMeta = userMeta
}

type FileMetaList []*FileMeta

func (m FileMeta) String() string {
	if m.IsDeleted {
		return fmt.Sprintf("%s: %s (deleted)", m.ID, m.Name)
	}
	return fmt.Sprintf("%s: %s (%db->%v)", m.ID, m.Name, m.Size, m.IsSymlink)
}

type ConsistencyLevel int

const (
	// ConsistencyLocal flags file for local persistence only, implying
	// that the file body will be stored on a single node. Default.
	ConsistencyLocal ConsistencyLevel = 0
	// ConsistencyS3 flags file for local+S3 persistence, implying that the file
	// body will be stored on a single node and Amazon S3.
	ConsistencyS3 ConsistencyLevel = 1
	// ConsistencyFull flags file to be replicated across all existing nodes in cluster and S3.
	ConsistencyFull ConsistencyLevel = 2
)

type ID string

type JournalMeta struct {
	ID         ID     `msgp:"0" json:"journal_id"`
	CreatedAt  int64  `msgp:"1" json:"created_at"`
	JoinedAt   int64  `msgp:"2" json:"joined_at"`
	FirstKey   string `msgp:"3" json:"first_key"`
	LastKey    string `msgp:"4" json:"last_key"`
	CountTotal int    `msgp:"5" json:"count_total"`
}

func (j JournalMeta) String() string {
	if len(j.FirstKey) == 0 {
		j.FirstKey = "?"
	}
	if len(j.LastKey) == 0 {
		j.LastKey = "?"
	}
	ts := time.Unix(0, j.CreatedAt).UTC().Format(time.StampMilli)
	return fmt.Sprintf("%s (%s): %s-%s (count: %d) joined: %v",
		j.ID, ts, j.FirstKey, j.LastKey, j.CountTotal, j.JoinedAt > 0)
}

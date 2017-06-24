package journal

import "fmt"

type ID string

type FileMeta struct {
	ID        ID          `msgp:"0" json:"file_id"  `
	Name      string      `msgp:"1" json:"file_name"`
	Size      int64       `msgp:"2" json:"file_size"`
	Timestamp int64       `msgp:"3" json:"timestamp"`
	UserMeta  interface{} `msgp:"4" json:"user_meta"`
	IsSymlink bool        `msgp:"5" json:"is_symlink"`

	Consistency ConsistencyLevel `msgp:"6" json:"consistency"`
}

func (m FileMeta) String() string {
	return fmt.Sprintf("%s: %s (%db->%v)", m.ID, m.Name, m.Size, m.IsSymlink)
}

type ConsistencyLevel int

const (
	// ConsistencyLocal flags file for local persistence only, implying
	// that the file body will be stored on a single node.
	ConsistencyLocal ConsistencyLevel = 0
	// ConsistencyS3 flags file for local+S3 persistence, implying that the file
	// body will be stored on a single node and Amazon S3.
	ConsistencyS3 ConsistencyLevel = 1
	// ConsistencyFull flags file to be replicated across all existing nodes in cluster and S3.
	ConsistencyFull ConsistencyLevel = 2
)

type JournalMeta struct {
	ID         ID     `msgp:"0" json:"journal_id"`
	CreatedAt  int64  `msgp:"1" json:"created_at"`
	JoinedAt   int64  `msgp:"2" json:"joined_at"`
	FirstKey   string `msgp:"3" json:"first_key"`
	LastKey    string `msgp:"4" json:"last_key"`
	CountTotal int    `msgp:"5" json:"count_total"`
}

func (j JournalMeta) String() string {
	return fmt.Sprintf("%s: %s-%s (%d) joined: %v", j.ID, j.FirstKey, j.LastKey, j.CountTotal, j.JoinedAt > 0)
}

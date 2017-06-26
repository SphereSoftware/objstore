package storage

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// LocalStorage provides access to the local filesystem.
type LocalStorage interface {
	Prefix() string
	Read(key string) (*os.File, error)
	Stat(key string) (os.FileInfo, error)
	Delete(key string) error
	Write(key string, body io.Reader) (int64, error)
	ListFiles(prefix string) ([]os.FileInfo, error)
	CheckAccess(prefix string) error
	DiskStats() (*DiskStats, error)
}

type localStorage struct {
	prefix string
}

func NewLocalStorage(prefix string) LocalStorage {
	return &localStorage{
		prefix: prefix,
	}
}

func (l *localStorage) Prefix() string {
	return l.prefix
}

func (l *localStorage) Read(key string) (*os.File, error) {
	return os.OpenFile(filepath.Join(l.prefix, key), os.O_RDONLY, 0600)
}

func (l *localStorage) Stat(key string) (os.FileInfo, error) {
	return os.Stat(filepath.Join(l.prefix, key))
}

func (l *localStorage) Delete(key string) error {
	return os.Remove(filepath.Join(l.prefix, key))
}

func (l *localStorage) Write(key string, body io.Reader) (int64, error) {
	f, err := os.OpenFile(filepath.Join(l.prefix, key), os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0600)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.Copy(f, body)
}

func (l *localStorage) ListFiles(path string) ([]os.FileInfo, error) {
	var infos []os.FileInfo
	path = filepath.Join(l.prefix, path)
	err := filepath.Walk(path, func(name string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		} else if info.IsDir() {
			if path == name {
				return nil
			}
			return filepath.SkipDir
		}
		infos = append(infos, info)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return infos, nil
}

func (l *localStorage) CheckAccess(path string) error {
	body := []byte(time.Now().UTC().String())
	key := filepath.Join(path, "_objstore_touch")
	_, err := l.Write(key, bytes.NewReader(body))
	return err
}

type DiskStats struct {
	BytesAll  uint64 `json:"bytes_all"`
	BytesUsed uint64 `json:"bytes_used"`
	BytesFree uint64 `json:"bytes_free"`
}

func (l *localStorage) DiskStats() (*DiskStats, error) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(l.prefix, &fs); err != nil {
		return nil, err
	}
	ds := &DiskStats{
		BytesAll:  fs.Blocks * uint64(fs.Bsize),
		BytesFree: fs.Bfree * uint64(fs.Bsize),
	}
	ds.BytesUsed = ds.BytesAll - ds.BytesFree
	return ds, nil
}

package storage

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"time"
)

// LocalStorage provides access to the local filesystem.
type LocalStorage interface {
	Prefix() string
	Read(key string) (*os.File, error)
	Stat(key string) (os.FileInfo, error)
	Write(key string, body io.Reader) error
	List(prefix string) ([]os.FileInfo, error)
	CheckAccess(prefix string) error
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

func (l *localStorage) Write(key string, body io.Reader) error {
	f, err := os.OpenFile(filepath.Join(l.prefix, key), os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, body)
	return err
}

func (l *localStorage) List(path string) ([]os.FileInfo, error) {
	var infos []os.FileInfo
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
	body := []byte(time.Now().String())
	return l.Write(filepath.Join(l.prefix, path, "_objstore_touch"), bytes.NewReader(body))
}

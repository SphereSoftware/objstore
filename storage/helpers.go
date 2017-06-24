package storage

import (
	"bytes"
	"io"
)

type readSeeker struct {
	io.Reader

	buf    []byte
	offset int
}

func newReadSeeker(buf []byte) io.ReadSeeker {
	return &readSeeker{
		Reader: bytes.NewReader(buf),
		buf:    buf,
	}
}

func (r *readSeeker) Seek(off int64, whence int) (int64, error) {
	offset := int(off)
	switch whence {
	case io.SeekStart:
		if offset < 0 || offset > len(r.buf) {
			return 0, io.EOF
		}
		r.offset = offset
		r.Reader = bytes.NewReader(r.buf[offset:])
	case io.SeekEnd:
		if offset < 0 || offset > len(r.buf) {
			return 0, io.EOF
		}
		r.offset = len(r.buf) - offset
		r.Reader = bytes.NewReader(r.buf[len(r.buf)-offset:])
	case io.SeekCurrent:
		if offset+r.offset > len(r.buf) ||
			offset+r.offset < 0 {
			return 0, io.EOF
		}
		r.offset = r.offset + offset
		r.Reader = bytes.NewReader(r.buf[r.offset:])
	default:
		panic("wrong whence arg")
	}
	return int64(r.offset), nil
}

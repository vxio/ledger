package log

import (
	"bufio"
	"encoding/binary"
	"os"
	"sync"
)

const (
	// number of bytes used to store the record's length
	lenWidth = 8
)

var (
	enc = binary.BigEndian
)

func newStore(f *os.File) (*store, error) {
	fi, err := os.Stat(f.Name())
	if err != nil {
		return nil, err
	}

	return &store{
		File: f,
		size: uint64(fi.Size()),
		buf:  bufio.NewWriter(f),
	}, nil
}

// wrapper API to append to and read from a file
type store struct {
	*os.File
	mu sync.Mutex
	// used a buffered writer to reduce # of system calls (disk writes) for better performance
	buf  *bufio.Writer
	size uint64
}

// persists input bytes to the file store
func (s *store) Append(p []byte) (n uint64, pos uint64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pos = s.size

	err = binary.Write(s.buf, enc, uint64(len(p)))
	if err != nil {
		return 0, 0, err
	}

	w, err := s.buf.Write(p)
	if err != nil {
		return 0, 0, err
	}

	w += lenWidth
	s.size += uint64(w)
	return uint64(w), pos, nil
}

// at the given position pos, return the record
func (s *store) ReadAt(pos uint64) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// append buffered data to the file
	if err := s.buf.Flush(); err != nil {
		return nil, err
	}

	recordLength := make([]byte, lenWidth)
	if _, err := s.File.ReadAt(recordLength, int64(pos)); err != nil {
		return nil, err
	}

	// read record's content
	b := make([]byte, enc.Uint64(recordLength))
	if _, err := s.File.ReadAt(b, int64(pos+lenWidth)); err != nil {
		return nil, err
	}

	return b, nil
}

// Close makes sure we persist buffered data before closing the file
func (s *store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.buf.Flush()
	if err != nil {
		return err
	}

	return s.File.Close()
}

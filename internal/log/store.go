package log

import (
	"bufio"
	"encoding/binary"
	"os"
	"sync"
)

var enc = binary.BigEndian

// defines the # of bytes used to store the record's length
const lenWidth = 8 // 64 bits

type store struct {
	*os.File
	mu sync.Mutex
	// we write to the buffered writer instead of directly to the file to reduce # of system calls in case a user
	// wrote a lot of small records
	buf  *bufio.Writer
	size uint64
}

func newStore(f *os.File) (*store, error) {
	fi, err := os.Stat(f.Name())
	if err != nil {
		return nil, err
	}

	size := uint64(fi.Size())
	return &store{
		File: f,
		size: size,
		buf:  bufio.NewWriter(f),
	}, nil
}

// returns # of bytes written, starting position (index) of the write, and err
func (this *store) Append(p []byte) (uint64, uint64, error) {
	this.mu.Lock()
	defer this.mu.Unlock()

	pos := this.size
	// write the length of the record, so when we read the record, we know how many bytes to read
	err := binary.Write(this.buf, enc, uint64(len(p)))
	if err != nil {
		return 0, 0, err
	}

	w, err := this.buf.Write(p)
	if err != nil {
		return 0, 0, err
	}
	w += lenWidth
	this.size += uint64(w)

	return uint64(w), pos, nil
}

func (this *store) ReadAt(pos uint64) ([]byte, error) {
	this.mu.Lock()
	defer this.mu.Unlock()
	err := this.buf.Flush()
	if err != nil {
		return nil, err
	}
	// find out how many bytes we have to read to get the whole record
	size := make([]byte, lenWidth)
	_, err = this.File.ReadAt(size, int64(pos))
	if err != nil {
		return nil, err
	}

	// fetch and return the record
	b := make([]byte, enc.Uint64(size)) // create byte slice with capcity = size
	_, err = this.File.ReadAt(b, int64(pos+lenWidth))
	if err != nil {
		return nil, err
	}

	return b, nil
}

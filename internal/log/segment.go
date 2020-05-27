package log

import (
	"fmt"
	"os"
	"path"

	"github.com/gogo/protobuf/proto"

	api "proglog/api/v1"
)

type segment struct {
	store                  *store
	index                  *index
	baseOffset, nextOffset uint64
	// used to check config limits and lets us know when our segment is maxed out
	config Config
}

func newSegment(dir string, baseOffset uint64, c Config) (*segment, error) {
	s := &segment{
		baseOffset: baseOffset,
		config:     c,
	}

	var err error
	storeFile, err := os.OpenFile(
		path.Join(dir, fmt.Sprintf("%d%s", baseOffset, ".store")),
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		0644,
	)
	if err != nil {
		return nil, err
	}

	s.store, err = newStore(storeFile)
	if err != nil {
		return nil, err
	}

	indexFile, err := os.OpenFile(
		path.Join(dir, fmt.Sprintf("%d%s", baseOffset, ".index")),
		os.O_RDWR|os.O_CREATE,
		0644,
	)
	if err != nil {
		return nil, err
	}
	s.index, err = newIndex(indexFile, c)
	if err != nil {
		return nil, err
	}

	// check if the current index is empty
	off, _, err := s.index.Read(-1)
	if err != nil { // empty index file
		s.nextOffset = baseOffset
	} else {
		s.nextOffset = baseOffset + uint64(off) + 1
	}

	return s, nil
}

func (this *segment) Append(record *api.Record) (uint64, error) {
	p, err := proto.Marshal(record)
	if err != nil {
		return 0, err
	}

	_, pos, err := this.store.Append(p)
	if err != nil {
		return 0, err
	}

	// index offsets are relative to the base offset
	offset := uint32(this.nextOffset - this.baseOffset)
	err = this.index.Write(offset, pos)
	if err != nil {
		return 0, err
	}

	resultOffset := this.nextOffset
	this.nextOffset += 1

	return resultOffset, nil
}

func (this *segment) Read(off uint64) (*api.Record, error) {
	// translate absolute offset to relative offset
	_, pos, err := this.index.Read(int64(off - this.baseOffset))
	if err != nil {
		return nil, err
	}

	b, err := this.store.ReadAt(pos)
	if err != nil {
		return nil, err
	}

	var record *api.Record
	err = proto.Unmarshal(b, record)
	if err != nil {
		return nil, err
	}

	return record, nil
}

// determines whether we need to create a new segment
func (this *segment) IsMaxed() bool {
	return this.store.size >= this.config.Segment.MaxStoreBytes ||
		this.index.size >= this.config.Segment.MaxIndexBytes
}

func (this *segment) Remove() error {
	err := this.Close()
	if err != nil {
		return err
	}

	filenames := []string{this.store.Name(), this.index.Name()}
	for _, filename := range filenames {
		err := os.Remove(filename)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *segment) Close() error {
	if err := s.index.Close(); err != nil {
		return err
	}
	if err := s.store.Close(); err != nil {
		return err
	}
	return nil
}

// returns the nearest and lesser multiple of k in j
// utility method to ensure we operate under the user's disk capacity
func nearestMultiple(j, k uint64) uint64 {

	if j >= 0 {
		return (j / k) * k
	}

	return ((j - k + 1) / k) * k
}

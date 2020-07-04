package log

import (
	"fmt"
	"os"
	"path"

	"github.com/gogo/protobuf/proto"

	api "ledger/api/v1"
)

// Segment handles operations between the store and index
type segment struct {
	store *store
	index *index

	baseOffset uint64
	nextOffset uint64

	config Config
}

func newSegment(dir string, baseOffset uint64, c Config) (*segment, error) {
	s := &segment{
		baseOffset: baseOffset,
		config:     c,
	}
	storeFile, err := os.OpenFile(
		//  filaname is {baseOffset}.store, e.g. 10.store
		path.Join(dir, fmt.Sprintf("%d%s", baseOffset, ".store")),
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		0644,
	)
	if err != nil {
		return nil, err
	}
	if s.store, err = newStore(storeFile); err != nil {
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
	if s.index, err = newIndex(indexFile, c); err != nil {
		return nil, err
	}
	if off, _, err := s.index.Read(-1); err != nil {
		s.nextOffset = baseOffset
	} else {
		s.nextOffset = baseOffset + uint64(off) + 1
	}
	return s, nil
}

// Appends the record and returns the record's offset
func (s *segment) Append(record *api.Record) (offset uint64, err error) {
	record.Offset = s.nextOffset
	b, err := proto.Marshal(record)
	if err != nil {
		return 0, err
	}

	_, pos, err := s.store.Append(b)
	if err != nil {
		return 0, err
	}
	err = s.index.Write(
		// index offsets are relative to base offset
		uint32(s.nextOffset-s.baseOffset),
		pos,
	)
	if err != nil {
		return 0, err
	}

	cur := s.nextOffset
	s.nextOffset += 1
	return cur, nil
}

// Find the record by offset
func (s *segment) Read(offset uint64) (*api.Record, error) {
	_, pos, err := s.index.Read(int64(offset - s.baseOffset))
	if err != nil {
		return nil, err
	}
	p, err := s.store.ReadAt(pos)
	if err != nil {
		return nil, err
	}
	record := &api.Record{}
	err = proto.Unmarshal(p, record)
	return record, err
}

// Determins whether the segment has reached its max size
func (s *segment) IsMaxed() bool {
	return s.store.size >= s.config.Segment.MaxStoreBytes ||
		s.index.size >= s.config.Segment.MaxIndexBytes
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

// Close the segment and removes all index and store files
func (s *segment) Remove() error {
	if err := s.Close(); err != nil {
		return err
	}
	if err := os.Remove(s.index.Name()); err != nil {
		return err
	}
	if err := os.Remove(s.store.Name()); err != nil {
		return err
	}
	return nil
}

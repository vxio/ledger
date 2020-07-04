package log

import (
	"io"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	api "ledger/api/v1"
)

// Ordered, append-only, write-ahead log
type Log struct {
	mu sync.RWMutex

	Dir    string
	Config Config

	// pointer to the active segment to append to
	activeSegment *segment
	// list of segments
	segments []*segment
}

func NewLog(dir string, c Config) (*Log, error) {
	if c.Segment.MaxStoreBytes == 0 {
		c.Segment.MaxStoreBytes = 1024
	}
	if c.Segment.MaxIndexBytes == 0 {
		c.Segment.MaxIndexBytes = 1024
	}
	l := &Log{
		Dir:    dir,
		Config: c,
	}

	// load existing log files if they exist
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var baseOffsets []uint64
	for _, file := range files {
		// get the
		offStr := strings.TrimSuffix(file.Name(), path.Ext(file.Name()))
		off, _ := strconv.ParseUint(offStr, 10, 0)
		baseOffsets = append(baseOffsets, off)
	}
	sort.Slice(baseOffsets, func(i, j int) bool {
		return baseOffsets[i] < baseOffsets[j]
	})
	for i := 0; i < len(baseOffsets); i++ {
		if err = l.newSegment(baseOffsets[i]); err != nil {
			return nil, err
		}
		// baseOffset contains dup for index and store so we skip the dup
		i++
	}
	if l.segments == nil {
		if err = l.newSegment(c.Segment.InitialOffset); err != nil {
			return nil, err
		}
	}
	return l, nil
}

// append the record to the log and return its offset
func (l *Log) Append(record *api.Record) (uint64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	off, err := l.activeSegment.Append(record)
	if err != nil {
		return 0, err
	}
	// if the segment is at its max size, allocate a new segment
	if l.activeSegment.IsMaxed() {
		err = l.newSegment(off + 1)
	}
	return off, err
}

// get a record by offset
func (l *Log) Read(offset uint64) (*api.Record, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var s *segment
	// segments are ordered oldest to newest
	for _, segment := range l.segments {
		if segment.baseOffset <= offset {
			s = segment
			break
		}
	}

	if s == nil || s.nextOffset <= offset {
		return nil, api.ErrOffsetOutOfRange{Offset: offset}
	}
	return s.Read(offset)
}

func (l *Log) LowestOffset() (uint64, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.segments[0].baseOffset, nil
}

func (l *Log) HighestOffset() (uint64, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	off := l.segments[len(l.segments)-1].nextOffset
	if off == 0 {
		return 0, nil
	}
	return off - 1, nil
}

func (l *Log) Truncate(lowest uint64) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	var segments []*segment
	for _, s := range l.segments {
		if s.nextOffset <= lowest+1 {
			if err := s.Remove(); err != nil {
				return err
			}
			continue
		}
		segments = append(segments, s)
	}
	l.segments = segments
	return nil
}

func (l *Log) newSegment(off uint64) error {
	s, err := newSegment(l.Dir, off, l.Config)
	if err != nil {
		return err
	}
	l.segments = append(l.segments, s)
	l.activeSegment = s
	return nil
}

func (l *Log) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, segment := range l.segments {
		if err := segment.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (l *Log) Remove() error {
	if err := l.Close(); err != nil {
		return err
	}
	return os.RemoveAll(l.Dir)
}

// Resets the log by removing all of its contents and creating a new instance of log
func (l *Log) Reset() error {
	if err := l.Remove(); err != nil {
		return err
	}
	var err error
	new, err := NewLog(l.Dir, l.Config)
	if err != nil {
		return err
	}
	*l = *new
	return nil
}

// Returns an io.Reader to read the whole log
func (l *Log) Reader() io.Reader {
	l.mu.RLock()
	defer l.mu.RUnlock()
	readers := make([]io.Reader, len(l.segments))
	for i, segment := range l.segments {
		readers[i] = segment.store
	}
	return io.MultiReader(readers...)
}

package log

import (
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	api "proglog/api/v1"
)

type Log struct {
	mu            sync.RWMutex
	Dir           string
	Config        Config
	activeSegment *segment // where subsequent records are written to
	segments      []*segment
}

func (this *Log) newSegment(off uint64) error {
	s, err := newSegment(this.Dir, off, this.Config)
	if err != nil {
		return err
	}
	this.segments = append(this.segments, s)
	this.activeSegment = s
	return nil
}

func NewLog(dir string, c Config) (*Log, error) {
	// defaults
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

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var baseOffsets []uint64
	for _, file := range files {
		offStr := strings.TrimSuffix(
			file.Name(),
			path.Ext(file.Name()),
		)

		off, err := strconv.ParseUint(offStr, 10, 0)
		if err != nil {
			return nil, err
		}
		baseOffsets = append(baseOffsets, off)
	}

	sort.Slice(baseOffsets, func(i, j int) bool {
		// sort in dsc order
		return baseOffsets[i] < baseOffsets[j]
	})

	for i := 0; i < len(baseOffsets); i++ {
		err := l.newSegment(baseOffsets[i])
		if err != nil {
			return nil, err
		}

		// baseOffsets contains a duplicate for both index and store, so we skip the duplicate
		i++
	}

	if l.segments == nil {
		err := l.newSegment(c.Segment.InitialOffset)
		if err != nil {
			return nil, err
		}
	}

	return l, nil
}

func (this *Log) Append(record *api.Record) (uint64, error) {
	this.mu.Lock()
	defer this.mu.Unlock()
	off, err := this.activeSegment.Append(record)
	if err != nil {
		return 0, err
	}
	// if active segment is at max size, add another segment
	if this.activeSegment.IsMaxed() {
		err = this.newSegment(off + 1)
	}
	return off, nil
}

func (this *Log) Read(off uint64) (*api.Record, error) {
	this.mu.RLock()
	defer this.mu.RUnlock()

	var segment *segment
	for _, s := range this.segments {
		if s.baseOffset <= off {
			segment = s
			break
		}
	}
	if segment == nil || segment.nextOffset <= off {
		return nil, api.ErrOffsetOutOfRange{Offset: off}
	}

	return segment.Read(off)
}

func (this *Log) Close() error {
	this.mu.Lock()
	defer this.mu.Unlock()
	for _, s := range this.segments {
		err := s.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (this *Log) Remove() error {
	err := this.Close()
	if err != nil {
		return err
	}

	return os.RemoveAll(this.Dir)
}

func (this *Log) Reset() error {
	err := this.Remove()
	if err != nil {
		return nil
	}

	log, err := NewLog(this.Dir, this.Config)
	if err != nil {
		return nil
	}

	*this = *log
	return nil
}

func (this *Log) LowestOffset() (uint64, error) {
	this.mu.RUnlock()
	defer this.mu.RUnlock()

	return this.segments[0].baseOffset, nil
}

func (this *Log) HighestOffset() (uint64, error) {
	this.mu.RLock()
	defer this.mu.RUnlock()
	off := this.segments[len(this.segments)-1].nextOffset
	if off == 0 {
		return 0, nil
	}

	return off, nil
}

// removes all segments whose highest offset is lower than the input 'lowest'
func (this *Log) Truncate(lowest uint64) error {
	this.mu.Lock()
	defer this.mu.Unlock()

	var segments []*segment
	for _, s := range this.segments {
		if s.nextOffset < lowest {
			err := s.Remove()
			if err != nil {
				return err
			}
			continue
		}
		segments = append(segments, s)
	}

	this.segments = segments
	return nil
}

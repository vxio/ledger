package log

import (
	"io"
	"os"

	"github.com/tysontate/gommap"
)

var (
	// offsets are stored as uint32
	offWidth uint64 = 4
	// positions are stored as uint64
	posWidth uint64 = 8
	// entWidth is used to jump to the position of an entry
	entWidth = offWidth + posWidth
)

type index struct {
	// persisted file
	file *os.File
	// memory-mapped file
	mmap gommap.MMap
	// the size of the index and where to write the next entry appended to the index
	size uint64
}

// creates an index from the given file
func newIndex(f *os.File, c Config) (*index, error) {
	idx := &index{
		file: f,
	}
	fi, err := os.Stat(f.Name())
	if err != nil {
		return nil, err
	}
	idx.size = uint64(fi.Size())
	// we grow the file here because we can't resize it once it is memory-mapped
	// there may be space between the last index entry and the end of the file
	// when we close the index, we must remove this empty space
	if err = os.Truncate(
		f.Name(), int64(c.Segment.MaxIndexBytes),
	); err != nil {
		return nil, err
	}
	if idx.mmap, err = gommap.Map(
		idx.file.Fd(),
		gommap.PROT_READ|gommap.PROT_WRITE,
		gommap.MAP_SHARED,
	); err != nil {
		return nil, err
	}
	return idx, nil
}

// ensures the memory-mapped file syncs its data to the persisted file and flushes its contents to the file before
// closing
func (i *index) Close() error {
	if err := i.mmap.Sync(gommap.MS_SYNC); err != nil {
		return err
	}
	if err := i.file.Sync(); err != nil {
		return err
	}
	// truncate the file based on amount of data that is in it
	if err := i.file.Truncate(int64(i.size)); err != nil {
		return err
	}
	return i.file.Close()
}

// Read accepts an offset and returns the record's offset and position in the store
//
// The offset is relative to the segment's base offset
// We use relative offsets to reduce the size of the indexes by storing offsets as uint32
//
// Return values:
// - record's offset
// - record's position
// - error
func (i *index) Read(in int64) (uint32, uint64, error) {
	if i.size == 0 {
		return 0, 0, io.EOF
	}
	// the index offset
	var offset uint64
	if in == -1 {
		offset = (i.size / entWidth) - 1
	} else {
		offset = uint64(in)
	}

	// position of the index entry
	indexPos := offset * entWidth

	// Attempted to read out of range of the index
	if i.size < indexPos+entWidth {
		return 0, 0, io.EOF
	}

	out := enc.Uint32(i.mmap[indexPos : indexPos+offWidth])
	pos := enc.Uint64(i.mmap[indexPos+offWidth : indexPos+entWidth])
	return out, pos, nil
}

// Appends the given offset and position to the index
func (i *index) Write(off uint32, pos uint64) error {
	// check if we have reached the file's size limit
	// does another index entry exceed the length of the memory-mapped file?
	if uint64(len(i.mmap)) < i.size+entWidth {
		return io.EOF
	}

	// appends the record's offset
	enc.PutUint32(i.mmap[i.size:i.size+offWidth], off)
	// appends the record's position
	enc.PutUint64(i.mmap[i.size+offWidth:i.size+entWidth], pos)

	i.size += entWidth

	return nil
}

func (i *index) Name() string {
	return i.file.Name()
}

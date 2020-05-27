package log

import (
	"io"
	"os"

	"github.com/tysontate/gommap"
)

var (
	// defines the #of bytes that make up each index entry
	offWidth uint64 = 4 // 4 bytes -> width of record's offset
	posWidth uint64 = 8 // 8 bytes -> width of record's position
	// used to jump straight to the position of an entry given its offset
	entryWidth = offWidth + posWidth // entry width
)

type index struct {
	file *os.File    // persisted file
	mmap gommap.MMap // memory-mapped file
	size uint64      // size of the index and where to write the next entry appened to the index
}

func newIndex(f *os.File, c Config) (*index, error) {
	idx := &index{
		file: f,
	}

	fi, err := os.Stat(f.Name())
	if err != nil {
		return nil, err
	}
	idx.size = uint64(fi.Size())

	// grow the size of the file to ensure we don't get an out of bounds error and convert to memory-map
	// we resize now since once they're memory-mapped, we can't resize them later
	err = os.Truncate(f.Name(), int64(c.Segment.MaxIndexBytes))
	if err != nil {
		return nil, err
	}

	idx.mmap, err = gommap.Map(
		idx.file.Fd(),
		gommap.PROT_READ|gommap.PROT_WRITE,
		gommap.MAP_SHARED,
	)
	if err != nil {
		return nil, err
	}

	return idx, nil
}

func (this *index) Close() error {
	// make sure memory-mapped file's contents are synced to the persisted file
	err := this.mmap.Sync(gommap.MS_SYNC)
	if err != nil {
		return err
	}

	// make sure persisted file has flushed its contents to stable storage
	err = this.file.Sync()
	if err != nil {
		return err
	}

	// truncates the persisted file to what's actually in it
	err = this.file.Truncate(int64(this.size))
	if err != nil {
		return err
	}

	return this.file.Close()
}

// Read takes an offset and returns the associate record's
// out: offset in the store
// pos: position in the store
// err: error

// the given offset is relative to the segment's base offset
func (this *index) Read(in int64) (out uint32, pos uint64, err error) {
	if this.size == 0 {
		return 0, 0, io.EOF
	}

	var indexOffset uint32
	if in == -1 {
		// get last offset index
		indexOffset = uint32((this.size / entryWidth) - 1)
	} else {
		indexOffset = uint32(in)
	}

	indexPos := uint64(indexOffset) * entryWidth
	if this.size < indexPos+entryWidth { // check if we're within bounds of the file size
		return 0, 0, io.EOF
	}

	// this.mmap => []byte => [... (out)(pos) ... ]
	// (out)(pos) starts at index 'indexPos' in []bytes
	// where 'out' is 4 bytes and 'pos' is 8 bytes

	out = enc.Uint32(this.mmap[indexPos : indexPos+offWidth])
	pos = enc.Uint64(this.mmap[indexPos+offWidth : indexPos+entryWidth])

	return out, pos, nil
}

// appends the given offset and position to the index
func (this *index) Write(off uint32, pos uint64) error {
	// make sure we have space to write the entry
	if uint64(len(this.mmap)) < this.size+entryWidth {
		return io.EOF
	}

	enc.PutUint32(this.mmap[this.size:this.size+offWidth], off)
	enc.PutUint64(this.mmap[this.size+offWidth:this.size+entryWidth], pos)

	this.size += entryWidth

	return nil
}

func (this *index) Name() string {
	return this.file.Name()
}

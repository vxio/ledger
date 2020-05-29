package log

import (
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	api "proglog/api/v1"
)

func TestSegment(t *testing.T) {
	dir, err := ioutil.TempDir("", "segment-test")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	record := &api.Record{Value: []byte("hello")}

	c := Config{}
	c.Segment.MaxStoreBytes = 1024

	numEntries := 3
	c.Segment.MaxIndexBytes = entryWidth * uint64(numEntries)

	baseOffset := uint64(16)
	s, err := newSegment(dir, baseOffset, c)
	require.NoError(t, err)
	require.Equal(t, uint64(16), s.nextOffset)
	require.False(t, s.IsMaxed())

	for i := 0; i < (numEntries); i++ {
		offset, err := s.Append(record)
		require.NoError(t, err)
		require.Equal(t, baseOffset+uint64(i), offset)

		got, err := s.Read(offset)
		require.NoError(t, err)
		require.Equal(t, record, got)
	}

	// check that we've maxed out the current index
	_, err = s.Append(record)
	require.Equal(t, io.EOF, err)
	require.True(t, s.IsMaxed())

	c.Segment.MaxStoreBytes = uint64(len(record.Value) * numEntries)
	c.Segment.MaxIndexBytes = 1024
	// calling newSegment a second time with the same baseOffset and dir checks that we load the state from the
	// persisted index and store files
	s, err = newSegment(dir, baseOffset, c)
	require.NoError(t, err)
	// store should be maxed out here
	require.True(t, s.IsMaxed())
}

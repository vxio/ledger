package log

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	write = []byte("hello world")
	width = uint64(len(write)) + lenWidth
)

func TestStore(t *testing.T) {
	f, err := ioutil.TempFile("", "store_test")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	store, err := newStore(f)
	require.NoError(t, err)

	testAppend(t, store)
	testRead(t, store)
}

func testAppend(t *testing.T, s *store) {
	t.Helper()
	for i := uint64(1); i < 4; i++ {
		n, pos, err := s.Append(write)
		require.NoError(t, err)
		require.Equal(t, pos+n, width*i)
	}
}

func testRead(t *testing.T, s *store) {
	t.Helper()
	var pos uint64
	for i := uint64(1); i < 4; i++ {
		read, err := s.ReadAt(pos)
		require.NoError(t, err)

		require.Equal(t, write, read)
		pos += width
	}
}

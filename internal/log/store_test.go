package log

import (
	"io"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	recordValue = []byte("hello")
	appendWidth = uint64(len(recordValue)) + lenWidth
)

func TestStore(t *testing.T) {
	name := path.Join(os.TempDir(), "log_test")
	f, err := os.OpenFile(
		name,
		os.O_RDWR|os.O_CREATE|os.O_EXCL|os.O_APPEND,
		0600,
	)
	require.NoError(t, err)
	defer os.Remove(f.Name())

	s, err := newStore(f)
	require.NoError(t, err)

	numRecords := 5

	testAppend(t, s, numRecords)
	testRead(t, s, numRecords)

	// create new store based on file that's been written to
	s, err = newStore(f)
	require.NoError(t, err)
	testRead(t, s, numRecords)

	// test invalid read: EOF
	_, err = s.ReadAt(s.size + 10)
	require.Equal(t, err, io.EOF)
}

func testAppend(t *testing.T, s *store, numAppends int) {
	t.Helper()
	for i := 1; i <= numAppends; i++ {
		n, pos, err := s.Append(recordValue)
		require.NoError(t, err)
		require.Equal(t, pos+n, appendWidth*uint64(i))
	}
}

func testRead(t *testing.T, s *store, numReads int) {
	t.Helper()

	var pos uint64
	for i := 1; i <= numReads; i++ {
		read, err := s.ReadAt(pos)
		require.NoError(t, err)
		require.Equal(t, recordValue, read)
		pos += appendWidth
	}
}

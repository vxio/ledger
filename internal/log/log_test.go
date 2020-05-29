package log

import (
	"errors"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	api "proglog/api/v1"
)

func TestLog(t *testing.T) {
	tests := map[string]func(t *testing.T, log *Log){
		"append and read record":      testAppendRead,
		"offset out of range error":   testOutOfRangeError,
		"init with existing segments": testInitExisting,
	}

	for desc, fn := range tests {
		t.Run(desc, func(t *testing.T) {
			dir, err := ioutil.TempDir("", "log-test")
			require.NoError(t, err)
			defer os.RemoveAll(dir)

			c := Config{}
			c.Segment.MaxStoreBytes = 32
			log, err := NewLog(dir, c)
			require.NoError(t, err)
			fn(t, log)
		})
	}

}

// checks that the log initializes with prior log data
func testInitExisting(t *testing.T, log *Log) {
	numRecords := 5
	record := &api.Record{
		Value: []byte("hello"),
	}
	for i := 0; i < numRecords; i++ {
		_, err := log.Append(record)
		require.NoError(t, err)
	}
	require.NoError(t, log.Close())

	newLog, err := NewLog(log.Dir, log.Config)
	require.NoError(t, err)

	off, err := newLog.Append(record)
	require.NoError(t, err)

	require.Equal(t, uint64(numRecords), off)
}

func testAppendRead(t *testing.T, log *Log) {
	record := &api.Record{
		Value: []byte("hello"),
	}
	off, err := log.Append(record)
	require.NoError(t, err)

	readRecord, err := log.Read(off)
	require.NoError(t, err)

	require.Equal(t, record, readRecord)
}

func testOutOfRangeError(t *testing.T, log *Log) {
	read, err := log.Read(1)
	require.Nil(t, read)
	var outOfRangeError *api.ErrOffsetOutOfRange
	require.True(t, errors.As(err, &outOfRangeError))
	require.Equal(t, uint64(1), outOfRangeError.Offset)
}

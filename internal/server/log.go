package server

import (
	"errors"
	"sync"
)

type Log struct {
	mu      sync.Mutex
	records []Record
}

func NewLog() *Log {
	return &Log{}
}

func (this *Log) Append(record Record) (uint64, error) {
	this.mu.Lock()
	defer this.mu.Unlock()

	record.Offset = uint64(len(this.records))
	this.records = append(this.records, record)

	return record.Offset, nil
}

func (this *Log) Read(offset uint64) (Record, error) {
	this.mu.Lock()
	defer this.mu.Unlock()

	if int(offset) >= len(this.records) {
		return Record{}, ErrOffsetNotFound
	}

	return this.records[int(offset)], nil
}

var ErrOffsetNotFound = errors.New("offset not found")

type Record struct {
	Value  []byte `json:"value"`
	Offset uint64 `json:"offset"`
}

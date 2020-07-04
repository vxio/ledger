package log

import (
	"github.com/hashicorp/raft"
)

// Config to build the log or distributed log
type Config struct {
	// Raft configuration
	Raft struct {
		raft.Config
		StreamLayer *StreamLayer
		Bootstrap   bool
	}
	//
	Segment struct {
		// specify the initial offset of the log
		InitialOffset uint64
		// max size of a segment's store
		MaxStoreBytes uint64
		// max size of a store's index
		MaxIndexBytes uint64
	}
}

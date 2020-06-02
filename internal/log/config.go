package log

import (
	"github.com/hashicorp/raft"
)

type Config struct {
	Raft struct {
		raft.Config
		StreamLayer *StreamLayer
		Bootstrap   bool
	}
	Segment struct {
		InitialOffset uint64
		MaxStoreBytes uint64
		MaxIndexBytes uint64
	}
}

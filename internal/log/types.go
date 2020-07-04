package log

// RequestType identifies the request type so we know how to handle it
type RequestType uint8

const (
	AppendRequestType RequestType = 0
)

// Identifier to identify connection type when we multiplex Raft on the same port as our log gRPC requests
const RaftRPC = 1

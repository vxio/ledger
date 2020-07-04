package log

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/gogo/protobuf/proto"
	raftboltdb "github.com/hashicorp/raft-boltdb"

	"github.com/hashicorp/raft"

	api "ledger/api/v1"
)

func NewDistributedLog(dataDir string, config Config) (
	*DistributedLog,
	error,
) {
	l := &DistributedLog{
		config: config,
	}
	if err := l.setupLog(dataDir); err != nil {
		return nil, err
	}
	if err := l.setupRaft(dataDir); err != nil {
		return nil, err
	}
	return l, nil
}

// Fault-tolerant and scalable distributed log
type DistributedLog struct {
	config Config
	log    *Log
	raft   *raft.Raft
}

func (l *DistributedLog) setupLog(dataDir string) error {
	logDir := filepath.Join(dataDir, "log")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}
	var err error
	l.log, err = NewLog(logDir, l.config)
	return err
}

func (l *DistributedLog) setupRaft(dataDir string) error {
	fsm := &fsm{log: l.log}

	logDir := filepath.Join(dataDir, "raft", "log")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}
	logConfig := l.config
	// Set the intial offset to 1, required by Raft
	logConfig.Segment.InitialOffset = 1
	logStore, err := newLogStore(logDir, logConfig)
	if err != nil {
		return err
	}

	// Using raft-boltdb as the stable store
	// a stable store is used by Raft to store the cluster's config: servers in the cluster, their addresses, etc.
	stableStore, err := raftboltdb.NewBoltStore(
		filepath.Join(dataDir, "raft", "stable"),
	)
	if err != nil {
		return err
	}

	// `retain` specifies the number of snapshots we'll keep
	retain := 1
	snapshotStore, err := raft.NewFileSnapshotStore(
		filepath.Join(dataDir, "raft"),
		retain,
		os.Stderr,
	)
	if err != nil {
		return err
	}

	maxPool := 5
	timeout := 10 * time.Second
	transport := raft.NewNetworkTransport(
		l.config.Raft.StreamLayer,
		maxPool,
		timeout,
		os.Stderr,
	)

	config := raft.DefaultConfig()
	config.LocalID = l.config.Raft.LocalID
	if l.config.Raft.HeartbeatTimeout != 0 {
		config.HeartbeatTimeout = l.config.Raft.HeartbeatTimeout
	}
	if l.config.Raft.ElectionTimeout != 0 {
		config.ElectionTimeout = l.config.Raft.ElectionTimeout
	}
	if l.config.Raft.LeaderLeaseTimeout != 0 {
		config.LeaderLeaseTimeout = l.config.Raft.LeaderLeaseTimeout
	}
	if l.config.Raft.CommitTimeout != 0 {
		config.CommitTimeout = l.config.Raft.CommitTimeout
	}

	l.raft, err = raft.NewRaft(
		config,
		fsm,
		logStore,
		stableStore,
		snapshotStore,
		transport,
	)
	if err != nil {
		return err
	}
	if l.config.Raft.Bootstrap {
		config := raft.Configuration{
			Servers: []raft.Server{{
				ID:      config.LocalID,
				Address: transport.LocalAddr(),
			}},
		}
		err = l.raft.BootstrapCluster(config).Error()
	}
	return err
}

func (l *DistributedLog) Append(record *api.Record) (uint64, error) {
	res, err := l.apply(
		AppendRequestType,
		&api.ProduceRequest{Record: record},
	)
	if err != nil {
		return 0, err
	}
	return res.(*api.ProduceResponse).Offset, nil
}

// Tells Raft to apply the command, once there's a quorum and the command is committed
// the FSM appends the record to the log
func (l *DistributedLog) apply(reqType RequestType, req proto.Marshaler) (
	interface{},
	error,
) {
	// Use a buffer here since we're supporting multiple request types:
	// As of 07/02/2020, we're supporting only one request type, `append`
	//
	// Write the request type identifier to the beginning of the buffer
	var buf bytes.Buffer
	_, err := buf.Write([]byte{byte(reqType)})
	if err != nil {
		return nil, err
	}
	// marshal the request into bytes
	b, err := req.Marshal()
	if err != nil {
		return nil, err
	}
	// add request content to the buffer
	_, err = buf.Write(b)
	if err != nil {
		return nil, err
	}
	timeout := 10 * time.Second
	// apply a command to the FSM
	// it replicates the record and appends the record to the leader's log
	//
	// This command must only be called on the cluster leader (as required by Raft)
	future := l.raft.Apply(b, timeout)
	// an error indicates something went wrong with Raft's replication
	// note: future.Error() is blocking
	if future.Error() != nil {
		return nil, future.Error()
	}
	res := future.Response()
	if err, ok := res.(error); ok {
		return nil, err
	}
	return res, nil
}

// Weak consistency guarantee
func (l *DistributedLog) Read(offset uint64) (*api.Record, error) {
	return l.log.Read(offset)
}

// Adds the server to the Raft cluster
// Must be called by the leader server or Raft will error
func (l *DistributedLog) Join(id, addr string) error {
	configFuture := l.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		return err
	}
	serverID := raft.ServerID(id)
	serverAddr := raft.ServerAddress(addr)
	for _, srv := range configFuture.Configuration().Servers {
		if srv.ID == serverID || srv.Address == serverAddr {
			if srv.ID == serverID && srv.Address == serverAddr {
				// server has already joined
				return nil
			}
			// remove the existing server by serverID
			removeFuture := l.raft.RemoveServer(serverID, 0, 0)
			if err := removeFuture.Error(); err != nil {
				return err
			}
		}
	}
	addFuture := l.raft.AddVoter(serverID, serverAddr, 0, 0)
	if err := addFuture.Error(); err != nil {
		return err
	}
	return nil
}

// Remove the server from the cluster
// Must be run on the leader or Raft will trigger an Error
// Removing the leader triggers a re-election
func (l *DistributedLog) Leave(id, addr string) error {
	removeFuture := l.raft.RemoveServer(raft.ServerID(id), 0, 0)
	return removeFuture.Error()
}

// Blocks til the cluster has elected a leader or times outs
func (l *DistributedLog) WaitForLeader(timeout time.Duration) error {
	timeoutc := time.After(timeout)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-timeoutc:
			return fmt.Errorf("leader election timed out")
		case <-ticker.C:
			if l := l.raft.Leader(); l != "" {
				return nil
			}
		}
	}
}

func (l *DistributedLog) Close() error {
	f := l.raft.Shutdown()
	if err := f.Error(); err != nil {
		return err
	}
	return l.log.Close()
}

func (l *DistributedLog) GetServers() ([]*api.Server, error) {
	future := l.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return nil, err
	}

	var servers []*api.Server
	for _, server := range future.Configuration().Servers {
		servers = append(servers, &api.Server{
			Id:       string(server.ID),
			RpcAddr:  string(server.Address),
			IsLeader: l.raft.Leader() == server.Address,
		})
	}

	return servers, nil
}

var _ raft.FSM = (*fsm)(nil)

// Raft runs our business logic through the FSM using the Apply method
type fsm struct {
	log *Log
}

// Raft invokes this method after committing a log entry
func (l *fsm) Apply(record *raft.Log) interface{} {
	buf := record.Data
	reqType := RequestType(buf[0])
	switch reqType {
	case AppendRequestType:
		return l.applyAppend(buf[1:])
	}
	return nil
}

// unmarshals the record and append it to our local log file
func (l *fsm) applyAppend(b []byte) interface{} {
	var req api.ProduceRequest
	err := req.Unmarshal(b)
	if err != nil {
		return err
	}
	offset, err := l.log.Append(req.Record)
	if err != nil {
		return err
	}
	return &api.ProduceResponse{Offset: offset}
}

// Called periodically to snapshot its state
// Here, we are storing a snapshot of the entire log
func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	r := f.log.Reader()
	return &snapshot{reader: r}, nil
}

// Raft calls this to restore an FSM from a snapshot
func (f *fsm) Restore(r io.ReadCloser) error {
	// reset the log
	if err := f.log.Reset(); err != nil {
		return err
	}
	recordLength := make([]byte, lenWidth)
	var buf bytes.Buffer
	for { // loop til we hit End of File (io.EOF)
		// Read the record's size
		_, err := io.ReadFull(r, recordLength)
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		// copy the contents into buf
		size := int64(enc.Uint64(recordLength))
		if _, err = io.CopyN(&buf, r, size); err != nil {
			return err
		}

		// append the record to the current log
		record := &api.Record{}
		if err = record.Unmarshal(buf.Bytes()); err != nil {
			return err
		}
		if _, err = f.log.Append(record); err != nil {
			return err
		}

		buf.Reset()
	}
	return nil
}

var _ raft.FSMSnapshot = (*snapshot)(nil)

// Point-in-time snapshot of the FSM's state
// Raft calls Persist() on the FSMSnapshot implementation to write its state to some store
//
// The store could be in-memory, a file, or cloud storage (S3, GCS, etc...)
// Here, we're using a file store
type snapshot struct {
	reader io.Reader
}

func (s *snapshot) Persist(sink raft.SnapshotSink) error {
	if _, err := io.Copy(sink, s.reader); err != nil {
		_ = sink.Cancel()
		return err
	}
	return sink.Close()
}

func (s *snapshot) Release() {}

var _ raft.LogStore = (*logStore)(nil)

// Raft uses a managed log store to store the commands that called by FSM's Apply() method
// We create Raft's log store using our own log
type logStore struct {
	*Log
}

func newLogStore(dir string, c Config) (*logStore, error) {
	log, err := NewLog(dir, c)
	if err != nil {
		return nil, err
	}
	return &logStore{log}, nil
}

func (l *logStore) FirstIndex() (uint64, error) {
	return l.LowestOffset()
}

func (l *logStore) LastIndex() (uint64, error) {
	off, err := l.HighestOffset()
	return off, err
}

func (l *logStore) GetLog(index uint64, out *raft.Log) error {
	in, err := l.Read(index)
	if err != nil {
		return err
	}
	out.Data = in.Value
	out.Index = in.Offset
	out.Type = raft.LogType(in.Type)
	out.Term = in.Term
	return nil
}

func (l *logStore) StoreLog(record *raft.Log) error {
	return l.StoreLogs([]*raft.Log{record})
}
func (l *logStore) StoreLogs(records []*raft.Log) error {
	for _, record := range records {
		if _, err := l.Append(&api.Record{
			Value: record.Data,
			Term:  record.Term,
			Type:  uint32(record.Type),
		});
			err != nil {
			return err
		}
	}
	return nil
}

func (l *logStore) DeleteRange(min, max uint64) error {
	return l.Truncate(max)
}

var _ raft.StreamLayer = (*StreamLayer)(nil)

// StreamLayer used by Raft to connect Raft servers
type StreamLayer struct {
	ln net.Listener
	// we enable encrypted communication between servers with TLS, so we need the TLS configs
	serverTLSConfig *tls.Config
	peerTLSConfig   *tls.Config
}

func NewStreamLayer(
	ln net.Listener,
	serverTLSConfig,
	peerTLSConfig *tls.Config,
) *StreamLayer {
	return &StreamLayer{
		ln:              ln,
		serverTLSConfig: serverTLSConfig,
		peerTLSConfig:   peerTLSConfig,
	}
}

func (s *StreamLayer) Dial(addr raft.ServerAddress, timeout time.Duration) (net.Conn,
	error) {
	dialer := &net.Dialer{Timeout: timeout}
	var conn, err = dialer.Dial("tcp", string(addr))
	if err != nil {
		return nil, err
	}
	// write the identifier as the first byte to tell mux this is a raft rpc
	_, err = conn.Write([]byte{byte(RaftRPC)})
	if err != nil {
		return nil, err
	}
	if s.peerTLSConfig != nil {
		conn = tls.Client(conn, s.peerTLSConfig)
	}
	return conn, err
}

func (s *StreamLayer) Accept() (net.Conn, error) {
	conn, err := s.ln.Accept()
	if err != nil {
		return nil, err
	}
	b := make([]byte, 1)
	_, err = conn.Read(b)
	if err != nil {
		return nil, err
	}
	// identify that this is a Raft connection
	if bytes.Compare([]byte{byte(RaftRPC)}, b) != 0 {
		return nil, fmt.Errorf("not a raft rpc")
	}
	if s.serverTLSConfig != nil {
		return tls.Server(conn, s.serverTLSConfig), nil
	}
	return conn, nil
}

func (s *StreamLayer) Close() error {
	return s.ln.Close()
}

func (s *StreamLayer) Addr() net.Addr {
	return s.ln.Addr()
}

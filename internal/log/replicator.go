package log

import (
	"context"
	"log"
	"sync"

	"google.golang.org/grpc"

	api "proglog/api/v1"
)

type Replicator struct {
	DialOptions []grpc.DialOption
	LocalServer api.LogClient

	mu sync.Mutex
	// rpc addr -> done channel
	// chan struct{} is used to signal that we should no longer replicate from this server
	servers map[string]chan struct{}
	closed  bool
	close   chan struct{}
}

func (this *Replicator) Join(name, addr string) error {
	this.mu.Lock()
	defer this.mu.Unlock()
	this.init()

	if this.closed {
		return nil
	}
	_, ok := this.servers[addr]
	if ok {
		// already replicating, skip it
		return nil
	}
	this.servers[addr] = make(chan struct{})

	go this.join(addr, this.servers[addr])
	return nil
}

func (this *Replicator) join(addr string, leave chan struct{}) {
	cc, err := grpc.Dial(addr, this.DialOptions...)
	if err != nil {
		this.err(err)
		return
	}
	defer cc.Close()

	client := api.NewLogClient(cc)
	ctx := context.Background()
	// start reading all logs from the log server
	stream, err := client.ConsumeStream(ctx, &api.ConsumeRequest{Offset: 0})
	if err != nil {
		this.err(err)
		return
	}

	// stream which will output records to be saved locally
	records := make(chan *api.Record)

	go func() {
		for {
			recv, err := stream.Recv()
			if err != nil {
				this.err(err)
				return
			}
			records <- recv.Record
		}
	}()

	// consume logs and produce to the local server to save a copy
	for {
		select {
		case <-this.close:
			return
		case <-leave:
			return
		case record := <-records:
			_, err = this.LocalServer.Produce(ctx, &api.ProduceRequest{Record: record})
			if err != nil {
				this.err(err)
				return
			}
		}
	}
}

func (this *Replicator) Leave(name, addr string) error {
	this.mu.Lock()
	defer this.mu.Unlock()
	this.init()
	_, ok := this.servers[addr]
	if !ok {
		return nil
	}

	close(this.servers[addr])
	delete(this.servers, addr)
	return nil
}

func (this *Replicator) init() {
	if this.servers == nil {
		this.servers = make(map[string]chan struct{})
	}
	if this.close == nil {
		this.close = make(chan struct{})
	}
}

func (this *Replicator) err(err error) {
	log.Printf("[ERROR] proglog: %v", err)
}

func (this *Replicator) Close() error {
	this.mu.Lock()
	defer this.mu.Unlock()
	this.init()

	if this.closed {
		return nil
	}

	this.closed = true
	close(this.close)
	return nil
}

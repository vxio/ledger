package loadbalancer

import (
	"context"
	"fmt"
	"log"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"

	api "ledger/api/v1"
)

const Name = "ledger"

// type that will fulfill gRPC's resolver.Builder and resolver.Resolver interfaces
type Resolver struct {
	mu            sync.Mutex
	clientConn    resolver.ClientConn
	resolverConn  *grpc.ClientConn
	serviceConfig *serviceconfig.ParseResult
}

// build sets up a client connection to our server so the resolver can call the GetServers() api
func (r *Resolver) Build(target resolver.Target, cc resolver.ClientConn,
	opts resolver.BuildOptions) (resolver.Resolver, error) {
	resolver.Register(r)
	r.clientConn = cc
	var dialOpts []grpc.DialOption
	if opts.DialCreds != nil {
		dialOpts = append(dialOpts,
			grpc.WithTransportCredentials(opts.DialCreds),
		)
	}

	r.serviceConfig = r.clientConn.ParseServiceConfig(
		// todo: ideally, his should go in a json file
		fmt.Sprintf(`{"loadBalancingConfig":[{"%s":{}}]}`, Name),
	)

	var err error
	r.resolverConn, err = grpc.Dial(target.Endpoint, dialOpts...)
	if err != nil {
		return nil, err
	}

	r.ResolveNow(resolver.ResolveNowOptions{})
	return r, nil
}

func (r *Resolver) Scheme() string {
	return Name
}

var _ resolver.Builder = (*Resolver)(nil)

var _ resolver.Resolver = (*Resolver)(nil)

// gRPC calls this method to resolve the target, discover the servers, and update the client connection with the servers
func (r *Resolver) ResolveNow(resolver.ResolveNowOptions) {
	r.mu.Lock()
	defer r.mu.Unlock()

	client := api.NewLogClient(r.resolverConn)
	// get cluster and set on client connection attributes
	ctx := context.Background()
	res, err := client.GetServers(ctx, &api.GetServersRequest{})
	if err != nil {
		log.Printf("[ERROR] ledger: failed to resolve servers: %v", err)
		return
	}
	var addrs []resolver.Address
	for _, server := range res.Servers {
		addrs = append(addrs, resolver.Address{
			Addr: server.RpcAddr,
			// attributes are optional but useful
			// lets us know which server is the leader/follower
			Attributes: attributes.New(
				"is_leader",
				server.IsLeader,
			),
		})
	}
	// update state to inform the load balancer what servers it can choose from
	r.clientConn.UpdateState(resolver.State{
		Addresses:     addrs,
		ServiceConfig: r.serviceConfig,
	})
}

func (r *Resolver) Close() {
	if err := r.resolverConn.Close(); err != nil {
		log.Printf("[ERROR] ledger: failed to close conn: %v", err)
	}
}

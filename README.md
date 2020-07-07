Ledger
------
Ledger is a money transaction ledger built on top of a distributed write-ahead log. Transactions written to the log
 are ordered, immutable, and contains all transaction records. In more detail:
- The log acts as the primary source of truth. The service's database state is derived from the log — making the
 application resilient to faults 
- A replica of all transactions could be created from applying records from the log
- The distributed log could be used for broader use cases since it's an ordered, append-only data structure at its core

#### Built with:
- Go
- gRPC and protobuf (internal networked service communication)
- PostgreSQL (app database)
- Serf (service discovery)
- Raft (leader election and replication)

#### Motivation
I wrote a money transaction system to see how a log would be useful in scenarios where we'd need performance
, scalability, and durability.

But why incur this kind of overhead? Why not use the database as the source of truth?
In a distributed system, services manage their own database and communicate across the network. Faults may
 occur in the form of network, machine, or process failure. Operations may result in updates for multiple databases
 . Replication latency may cause follower nodes to have stale data. To mitigate these faults, a write-ahead log is
  useful for acting as the primary source of truth and tracking state changes. 

Logs are the core piece underlying many distributed data systems, other use cases
 include:
- Write-ahead logs in SQL or NoSQL stores
- Version control 
- Replicated state machines (I use the log in the Raft implementation)
- Event logging  
<br /> 


## Implementation Details
Basic description of what happens when the app creates a transaction record:
1. The app composes a money transaction 
1. The app sends the transaction data to the write-ahead log
1. The log receives the transaction record and persists it
    - Using the distributed version of the log, multiple copies of the transaction are saved to the log
1. The log sends the transaction record back to the app
1. The app saves the transaction to its own database

### gRPC and protobuf for network communication
Advantages:
- Consistent schemas and type-safety
    - Protobuf compiles to type-checked Go code used by both the client and server 
    - Backwards compatible versioning
- Performance:
    - Fast serialization when unmarshalling requests and marshalling responses using protocol buffers
    - Using a single, long-lasting TCP connection over a new connection for each request

### Service Discovery using Serf
Adding service discovery to the distributed log allows the service to automatically handle:
 - When a new server is added or removed from the cluster
 - Health checks for service instances and removes them if they're unresponsive
 - Unregistering services when they go offline
 
 [Serf](https://serf.io) is used as the embedded service discovery layer that will provide cluster membership, failure
  detection, and orchestration.
 
 An alternative would be to use a service discovery service like Consul (which also uses Serf under the hood). To
  implement service discovery with Serf, each service will be a Serf node.
 
### Replication and Consensus - How servers agree on shared state and tolerate failures
We use replication to make the log service more resistant to failures. We store multiple copies of the log data when
 there are multiple servers in a cluster.
 Replication prevents data loss when servers fail, but they do come at a cost in terms of infrastructure overhead and data storage.

Log replication is built on top of [Raft](https://github.com/hashicorp/raft); Raft is a library for providing
 consensus and uses a leader-follower system. 
All `append` commands will be replicated since we want to record every state
 change in the log. 
 
For each client write request, the leader adds the write command to its log and requests its followers to add the
 command to their own logs. When the majority of logs replicate the command, the leader considers the command
  committed. 
After the commit, the leader will execute the command with the finite state machine and respond to the client with
  the result. 
  
 The recommended number of servers in a Raft cluster is 3 and 5. A Raft cluster of 3 servers will tolerate a single
  server failure while a cluster of 5 will tolerate 2 server failures.
  
### Load-Balancing Strategy
The log is a single writer, multiple reader distributed service. The leader-server handles write requests and the
 follower
-servers handle read requests. 
  
### Issues
- Read-modify-write operations: issues arise with latency in which the data in the underlying store may not reflect the
 most recent update in the log
- Duplicate messages (what if the same record is sent to the log twice?)

  

# What is it
Distributed write-ahead-log (WAL) for a money transactions ledger written in Go

- records written to the log are ordered and immutable; and contains all transaction records
- using the log as the primary source of truth, and the service database state is derived from the log makes
 applications resilient to faults 
- a replica can be created from applying records from the log
- the distributed log could be used for broader use cases since at its core it's an ordered, append-only data structure

I wrote a money transaction system to see how a log would be useful in scenarios where we'd need performance
/scalability/durability, logs are the core piece underlying many distributed data systems, other use cases include:
- WAL in SQL or NoSQL stores
- version control 
- replication (I use the same log in my Raft implementation)
goals:
- performance
- scalability
- high availability / fault tolerance

## features
- take transaction data into the log client (with the server setup through the agent), and write to our distributed log
    - leader: make sure all transaction data is accurate and saved to the log  
    - Make sure replicas have saved the log data (the tests should be done in the appropriate place)
- save this transaction data in PostgreSql


## Issues
- read-modify-write operations (issues arise with latency and the data in our underlying store may not reflect the
 most recent update in the log)
- duplicate messages (what if the same record is sent to the log twice?)

## Operations
- our app gets a transaction request (not going to build)
- send to our distributed log (agent)
- agent receives the record and persists it
- our application get a ProduceResponse from the log
- our app applies the transaction to its database

## Service Discovery using Serf
What?
Service discovery allows our service to automatically handle:
 - when a new node/server is added or removed from our cluster
 - health checks service instances and remove them if they're unresponsive
 - deregister services when they go offline
 
 Serf - our embedded service discovery layer that will provide cluster membership, failure detection, and orchestration.
 
 An alternative would be to use a service discovery service like Consul (which also uses Serf under the hood).

 To implement service discovery with Serf, each of our services will essentially be a Serf node
 
## Consensus - how servers determine on shared state and tolerate failures
We use replication to make our service more resistent to failures. We'll store multiple copies of the log data when
 we have multiple servers in a cluster.
 Replication saves us from losing data when servers fail but they do come at a cost in terms of infrastructure
  overhead and data storage.

We use Raft for log replication; we'll be replicating the `append` commands since we want to record every state
 change in our log. 
Raft is a distributed consensus algorithm that uses a leader-follower system. 
 
For each client write request, the leader appends the command to its log and requests that its followers append the
 command to their logs. When the majority of logs replicate the command, the leader considers the command committed. 
 After the commit, the leader will execute the command with the finite state machine and respond to the client with
  the result. 
  The recommended number of servers in a Raft cluster is 3 and 5. A Raft cluster of 3 servers will tolerate a single server failure while a cluster of 5 will tolerate 2 server failures.
  
  Load-Balancing Strategy
  We're building a single writer, multiple reader distributed service. So the read requests are sent to the followers
   and the leader can focus on writes. 
  
  
  

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

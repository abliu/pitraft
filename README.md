pitraft
=====

## Overview

pitraft is an implementation of the game
[Pit](http://en.wikipedia.org/wiki/Pit_(game)) using the distributed consensus
protocol [Raft](https://ramcloud.stanford.edu/wiki/download/attachments/11370504/raft.pdf).

## Running

To start a game and play, do the following:

1. Install [Go](http://golang.org/).
2. Get the source code via ```go get https://github.com/abliu/pitraft```.
3. To start running a leader machine for the distributed consensus algorithm,
   first make a directory in which you want to store log files. Then, from
   command line, execute:
   ```cd <pitraft_repo_folder>
      go run main.go -p 4001 <your_log_file_folder>```
4. To start running a follower machine for the distributed consensus algorithm,
   first make a directory in which you want to store log files. Then, from
   command line, execute:
   ```cd <pitraft_repo_folder>
      go run main.go -p <desired_port> -join <host_address:port>
      <your_log_file_folder>```
   where `host_address:port` denotes the leader's address and port number,
   specified in -p in #3.
5. To start running a client machine that plays Pit, execute:
   ```cd <pitraft_repo_folder>/client
      go run client.go -u <host_address:port>```
   where `host_address:port` again denotes the leader's address and port number,
   specified in -p in #3.
6. When you are playing on the client machine, you will be playing through a
   command line shell. You can enter 

## Caveats

One issue with running a 2-node distributed consensus protocol is that we need both servers operational to make a quorum and to perform an actions on the server.
So if we kill one of the servers at this point, we will will not be able to update the system (since we can't replicate to a majority).
You will need to add additional nodes to allow failures to not affect the system.
For example, with 3 nodes you can have 1 node fail.
With 5 nodes you can have 2 nodes fail.


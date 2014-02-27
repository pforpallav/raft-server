raft-server
===========

A simple raft based cluster server package in golang.

Install
-------

To install the package in your GOPATH just run the following command:

  `$ go get github.com/pforpallav/raft-server`

Usage
-----

To use the library in your go-code import the package by adding this import line

  `import raft "github.com/pforpallav/raft-server"`
  
Now you can access the interfaces, structs and functions by through `raft` namespace.

Documentation
-------------

#### func AddRaftPeer
`func AddRaftPeer(id int, config string) Raft`

   id - Peer's id  
   config - .json file with configuration (sample file is raftConfig.json)  

Starts a new raft peer in the cluster. The peer listens to address corresponding to the id mentioned in the config file. Returns a Raft interface.

#### type Raft
`type Raft interface`

  `Term() int` - Return self's current Term.  
  `isLeader() []int` - Return whether self is the leader.  
  `Pause() bool` - Pause the peer (cut it off the cluster).  
  `Unpause() bool` - Join back the cluster.  

Sample Test
-----------
  `<src folder> $ go test`
  
Testing with five servers. Minority number of failures. Majority number of failures.
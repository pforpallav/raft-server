package raft

import {
	"fmt"
}

import cluster "github.com/pforpallav/cluster-server"

type Raft interface {
	// The current term for this peer
    Term()     int

    // The current leader according to this peer
    isLeader() bool
}

type RaftBody struct {

	isLeader bool
	peerObject cluster.Server

	//----Persistant State----//

	// Current term for this peer
	currentTerm int

	// Whom this peer voted for?
	votedFor int

	// Log
	//log []interface{}

	//-----Volatile State-----//

	// Index of highest log entry known to be committed (initialized to 0, increases monotonically)
	//commitIndex int

	// Index of highest log entry applied to state machine (initialized to 0, increases monotonically)
	//lastApplied int


	//----Volative State (Leader)----//

	// For each server, index of the next log entry to send to that server (initialized to leader last log index + 1)
	//nextIndex []int

	// For each server, index of highest log entry known to be replicated on server (initialized to 0, increases monotonically
	//matchIndex []int
}

func (r RaftBody) Term() int {
	return r.currentTerm
}

func (r RaftBody) isLeader() bool {
	return isLeader
}

func AddRaftPeer(id int, config string) Raft {

	type ConfigData struct {
		lowerElectionTO int //in ms
		upperElectionTO int //in ms

		numPeers int //total number of peers
		clusterConfig string //filename of the .json with file info
	}

	ConfigFile, err := ioutil.ReadFile(config)
	if err != nil {
		panic(err)
	}

	//Decoding into a ConfigData
	var c ConfigData
	err = json.Unmarshal(ConfigFile, &c)
	if err != nil {
		panic(err)
	}

	clusterObject := cluster.AddPeer(id, c.clusterConfig)

	Me := RaftBody{false, clusterObject, 0, 0}

	
}
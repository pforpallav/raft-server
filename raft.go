package raft

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"
	"math/rand"
	//"log"
)

import cluster "github.com/pforpallav/cluster-server"

type RaftMessage struct {
	msgType int // 1 for Request, 2 for Response
	term    int
	result2 bool
	id      int
	callTo  int // 1 for RequestVote, 2 for AppendEntries
}

type Raft interface {
	// The current term for this peer
	Term() int

	// The current leader according to this peer
	isLeader() bool
}

//Raft internals
type RaftPeer interface {
	// Method for requesting vote
	RequestVote(term int, candidateId int /*, lastLogIndex int, lastLogTerm int*/) (int, bool)

	// For appending entries and HeartBeats
	AppendEntries(term int, leaderId int /*, prevLogIndex int, entries []interface{}, leaderCommit int*/) (int, bool)

	// Runtime functions
	Runnable() int
}

type RaftBody struct {

	mode       string

	peerObject cluster.Server

	NumServers int

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
	return (r.mode == "L")
}

func (r RaftBody) RequestVote(term int, candidateId int /*, lastLogIndex int, lastLogTerm int*/) (int, bool) {
	if r.mode == "F" && r.currentTerm < term {
		r.currentTerm = term
		return r.currentTerm, true
	} else {
		return r.currentTerm, false
	}
}

func (r RaftBody) AppendEntries(term int, leaderId int /*, prevLogIndex int, entries []interface{}, leaderCommit int*/) (int, bool) {
	return r.currentTerm, true
}

func (r RaftBody) Runnable(HeartBeat int, LowerElectionTO int, UpperElectionTO int) int {

	totalVotes := 0
	var msg RaftMessage

	for {
		if r.mode == "L" {
			select {
			case e := <-r.peerObject.Inbox():
				msg = e.Msg.(RaftMessage)
				if msg.term > r.currentTerm {
					r.mode = "F"
				}
				if msg.msgType == 1 {
					if msg.callTo == 1 {
						t, vote := r.RequestVote(msg.term, msg.id)
						r.peerObject.Outbox() <- &cluster.Envelope{Pid: msg.id, Msg: RaftMessage{2, t, vote, r.peerObject.Pid(), 0}}
					} else if msg.callTo == 2 {
						t, taskDone := r.AppendEntries(msg.term, msg.id)
						r.peerObject.Outbox() <- &cluster.Envelope{Pid: msg.id, Msg: RaftMessage{2, t, taskDone, r.peerObject.Pid(), 0}}
					}
				}
			case <-time.After(time.Duration(HeartBeat) * time.Millisecond):
				r.peerObject.Outbox() <- &cluster.Envelope{Pid: -1, Msg: RaftMessage{1, r.currentTerm, false, r.peerObject.Pid(), 2}}
			}

		} else if r.mode == "C" {
			select {
			case e := <-r.peerObject.Inbox():
				msg = e.Msg.(RaftMessage)
				if msg.term > r.currentTerm {
					r.mode = "F"
				}
				if msg.msgType == 1 {
					if msg.callTo == 1 {
						t, vote := r.RequestVote(msg.term, msg.id)
						r.peerObject.Outbox() <- &cluster.Envelope{Pid: msg.id, Msg: RaftMessage{2, t, vote, r.peerObject.Pid(), 0}}
					} else if msg.callTo == 2 {
						t, taskDone := r.AppendEntries(msg.term, msg.id)
						r.peerObject.Outbox() <- &cluster.Envelope{Pid: msg.id, Msg: RaftMessage{2, t, taskDone, r.peerObject.Pid(), 0}}
					}
				} else if msg.msgType == 2 {
					if msg.callTo == 1 {
						if msg.result2 {
							totalVotes++
							if totalVotes > r.NumServers/2 && r.mode == "C" {
								r.mode = "L"
								fmt.Printf("Peer %d is now the leader! Yo!\n", r.peerObject.Pid())
							}
						}
					}
				}
			case <-time.After(time.Duration(rand.Intn(UpperElectionTO-LowerElectionTO) + LowerElectionTO) * 1000 * time.Nanosecond):
				r.currentTerm++
				totalVotes = 1
				r.peerObject.Outbox() <- &cluster.Envelope{Pid: -1, Msg: RaftMessage{1, r.currentTerm, false, r.peerObject.Pid(), 1}}
			}
		} else if r.mode == "F" {
			select {
			case e := <-r.peerObject.Inbox():
				msg = e.Msg.(RaftMessage)
				if msg.msgType == 1 {
					if msg.callTo == 1 {
						t, vote := r.RequestVote(msg.term, msg.id)
						r.peerObject.Outbox() <- &cluster.Envelope{Pid: msg.id, Msg: RaftMessage{2, t, vote, r.peerObject.Pid(), 0}}
					} else if msg.callTo == 2 {
						t, taskDone := r.AppendEntries(msg.term, msg.id)
						r.peerObject.Outbox() <- &cluster.Envelope{Pid: msg.id, Msg: RaftMessage{2, t, taskDone, r.peerObject.Pid(), 0}}
					}
				}
			case <-time.After(time.Duration(rand.Intn(UpperElectionTO-LowerElectionTO) + LowerElectionTO) * 1000 * time.Nanosecond):
				fmt.Printf("Peer %d turning to candidate\n", r.peerObject.Pid())
				r.mode = "C"
				r.currentTerm++
				totalVotes = 1
				r.peerObject.Outbox() <- &cluster.Envelope{Pid: -1, Msg: RaftMessage{1, r.currentTerm, false, r.peerObject.Pid(), 1}}
			}
		}
	}
}

func AddRaftPeer(id int, config string) Raft {

	type ConfigData struct {
		HeartBeat       int //in ms
		LowerElectionTO int //in ms
		UpperElectionTO int //in ms
		NumPeers      int    //total number of peers
		ClusterConfig string //filename of the .json with cluster info
	}

	ConfigFile, err := ioutil.ReadFile(config)
	if err != nil {
		panic(err)
	}

	//fmt.Printf("%s\n", ConfigFile)
	//Decoding into a ConfigData
	var c ConfigData
	err = json.Unmarshal(ConfigFile, &c)
	if err != nil {
		panic(err)
	}

	//fmt.Printf("%d %d %d %d %s\n", c.HeartBeat, c.LowerElectionTO, c.UpperElectionTO, c.NumPeers, c.ClusterConfig)

	clusterObject := cluster.AddPeer(id, c.ClusterConfig)

	Me := RaftBody{"F", clusterObject, c.NumPeers, 0, 0}

	rand.Seed(100)

	go Me.Runnable(c.HeartBeat, c.LowerElectionTO, c.UpperElectionTO)

	return Me
}

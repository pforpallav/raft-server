package raft

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"time"
	//"log"
)

import cluster "github.com/pforpallav/cluster-server"

const (
	DEBUG = 0
)

type RaftMessage struct {
	MsgType int // 1 for Request, 2 for Response
	Term    int
	Result2 bool
	Id      int
	CallTo  int // 1 for RequestVote, 2 for AppendEntries
}

type LeaderInfo struct {
	LeaderId     int
	Term         int
	MajorityFrom string
}

type Raft interface {
	// The current term for this peer
	Term() int

	// The current leader according to this peer
	isLeader() bool

	// Mailbox for state machine layer above to send commands of any
	// kind, and to have them replicated by raft.  If the server is not
	// the leader, the message will be silently dropped.
	Outbox() chan<- interface{}

	// Mailbox for state machine layer above to receive commands. These
	// are guaranteed to have been replicated on a majority
	Inbox() <-chan *LogItem

	// Remove items from 0 .. index (inclusive), and reclaim disk
	// space. This is a hint, and there's no guarantee of immediacy since
	// there may be some servers that are lagging behind).
	DiscardUpto(index int64)

	// Pause/Unpause - drop messages recieved and to be sent
	Pause() bool
	Unpause() bool
}

// Identifies an entry in the log
type LogEntry struct{
	// An index into an abstract 2^64 size array
	Index  int64

	// The data that was supplied to raft's inbox
	Data    interface{}
}

// Statemachine interface
type SM interface {
	
	// Action to execute on the State Machine
	Execute(action interface{})
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
	// Current mode of operation
	mode string

	// Cluster object
	peerObject cluster.Server

	// Total number of peers in the cluster
	NumServers int

	// Last majority achieved from:
	votesFrom string

	// Total number of votes recieved till now in C mode
	totalVotes int

	//----Persistant State----//

	// Current term for this peer
	currentTerm int

	// Whom this peer voted for?
	votedFor int

	// Log
	log []LogEntry

	//-----Volatile State-----//

	// Index of highest log entry known to be committed (initialized to 0, increases monotonically)
	commitIndex int

	// Index of highest log entry applied to state machine (initialized to 0, increases monotonically)
	lastApplied int

	//----Volative State (Leader)----//

	// For each server, index of the next log entry to send to that server (initialized to leader last log index + 1)
	nextIndex []int

	// For each server, index of highest log entry known to be replicated on server (initialized to 0, increases monotonically
	matchIndex []int
}

var LeaderChan chan *LeaderInfo //Send singnal if a new leader is elected
var LowerElectionTO int         //Lower Election Timeout
var UpperElectionTO int         //Upper Election Timeout
var HeartBeat int               //Heartbeat timeout

func init() {
	LeaderChan = make(chan *LeaderInfo)
}

func (r *RaftBody) Term() int {
	return r.currentTerm
}

func (r *RaftBody) isLeader() bool {
	return (r.mode == "L")
}

func (r *RaftBody) Pause() bool {
	return r.peerObject.Pause()
}

func (r *RaftBody) Unpause() bool {
	return r.peerObject.Unpause()
}

// RPC RequestVote
func (r *RaftBody) RequestVote(term int, candidateId int /*, lastLogIndex int, lastLogTerm int*/) (int, bool) {
	if (r.mode == "F") && (r.currentTerm < term) {
		r.mode = "F"
		r.currentTerm = term
		r.votedFor = candidateId
		if DEBUG == 1 {
			fmt.Printf("%d voting for %d for %d term\n", r.peerObject.Pid(), candidateId, term)
		}
		return r.currentTerm, true
	} else {
		return r.currentTerm, false
	}
}

// RPC AppendEntries
func (r *RaftBody) AppendEntries(term int, leaderId int /*, prevLogIndex int, entries []interface{}, leaderCommit int*/) (int, bool) {
	if r.currentTerm < term {
		r.mode = "F"
		if DEBUG == 1 {
			fmt.Printf("Term changed for peer %d from %d to %d\n", r.peerObject.Pid(), r.currentTerm, term)
		}
		r.currentTerm = term
	}
	return r.currentTerm, true
}

// goroutine for sending Heartbeats in L mode
func Heartbeat(r *RaftBody) {
	for {

		if r.mode != "L" {
			if DEBUG == 1 {
				fmt.Printf("Heart break for %d\n", r.peerObject.Pid())
			}
			break
		}
		select {
		case <-time.After(time.Duration(HeartBeat) * time.Millisecond):
			b, err := json.Marshal(RaftMessage{1, r.currentTerm, false, r.peerObject.Pid(), 2})
			if err != nil {
				panic(err)
			}
			r.peerObject.Outbox() <- &cluster.Envelope{Pid: -1, Msg: b}
		}

	}
}

// goroutine for the raft peer
func Runnable(r *RaftBody, HeartBeat int, LowerElectionTO int, UpperElectionTO int) int {

	r.totalVotes = 0
	r.votesFrom = ""
	var msg RaftMessage

	for {
		// Leader "L" mode
		if r.mode == "L" {
			select {
			case e := <-r.peerObject.Inbox():

				err := json.Unmarshal(e.Msg.([]byte), &msg)
				if err != nil {
					panic(err)
				}

				if msg.Term > r.currentTerm {
					if DEBUG == 1 {
						fmt.Printf("Higher term from %d to %d\n", msg.Id, r.peerObject.Pid())
					}

					r.mode = "F"
					r.currentTerm = msg.Term
					r.totalVotes = 0
					r.votesFrom = ""
				}

				if msg.MsgType == 1 {
					if msg.CallTo == 1 {
						t, vote := r.RequestVote(msg.Term, msg.Id)
						b, err := json.Marshal(RaftMessage{2, t, vote, r.peerObject.Pid(), 1})
						if err != nil {
							panic(err)
						}
						r.peerObject.Outbox() <- &cluster.Envelope{Pid: msg.Id, Msg: b}
					} else if msg.CallTo == 2 {
						t, taskDone := r.AppendEntries(msg.Term, msg.Id)
						b, err := json.Marshal(RaftMessage{2, t, taskDone, r.peerObject.Pid(), 2})
						if err != nil {
							panic(err)
						}
						r.peerObject.Outbox() <- &cluster.Envelope{Pid: msg.Id, Msg: b}
					}
				}
			}

		} else if r.mode == "C" {

			// Candidate "C" mode

			select {
			case e := <-r.peerObject.Inbox():
				err := json.Unmarshal(e.Msg.([]byte), &msg)
				if err != nil {
					panic(err)
				}

				if msg.Term > r.currentTerm {
					r.mode = "F"
					r.currentTerm = msg.Term
					r.totalVotes = 0
					r.votesFrom = ""
				}

				if msg.MsgType == 1 {
					if msg.CallTo == 1 {
						t, vote := r.RequestVote(msg.Term, msg.Id)
						b, err := json.Marshal(RaftMessage{2, t, vote, r.peerObject.Pid(), 1})
						if err != nil {
							panic(err)
						}
						r.peerObject.Outbox() <- &cluster.Envelope{Pid: msg.Id, Msg: b}
					} else if msg.CallTo == 2 {
						t, taskDone := r.AppendEntries(msg.Term, msg.Id)
						b, err := json.Marshal(RaftMessage{2, t, taskDone, r.peerObject.Pid(), 2})
						if err != nil {
							panic(err)
						}
						r.peerObject.Outbox() <- &cluster.Envelope{Pid: msg.Id, Msg: b}
					}
				} else if msg.MsgType == 2 {
					if msg.CallTo == 1 {
						if msg.Result2 == true && msg.Term == r.currentTerm {
							r.totalVotes++
							r.votesFrom += " " + string(msg.Id)

							if DEBUG == 1 {
								fmt.Printf("Got a vote from %d to %d\n", msg.Id, r.peerObject.Pid())
							}

							if r.totalVotes > r.NumServers/2 && r.mode == "C" {
								r.mode = "L"
								go Heartbeat(r)
								//fmt.Printf("%d \t %d \t %d \t %q\n", r.peerObject.Pid(), r.currentTerm, r.totalVotes, r.votesFrom)
								LeaderChan <- &LeaderInfo{r.peerObject.Pid(), r.currentTerm, r.votesFrom}
							}
						}
					} else if msg.CallTo == 2 {
						if msg.Term > r.currentTerm {
							r.mode = "F"
							r.currentTerm = msg.Term
							r.totalVotes = 0
							r.votesFrom = ""
						}
					}
				}
			case <-time.After(time.Duration(rand.Intn(UpperElectionTO-LowerElectionTO)+LowerElectionTO) * time.Millisecond):
				r.currentTerm++
				r.totalVotes = 1
				r.votesFrom = string(r.peerObject.Pid())
				if DEBUG == 1 {
					fmt.Printf("Peer %d turning to candidate for term %d\n", r.peerObject.Pid(), r.currentTerm)
				}

				b, err := json.Marshal(RaftMessage{1, r.currentTerm, false, r.peerObject.Pid(), 1})
				if err != nil {
					panic(err)
				}
				r.peerObject.Outbox() <- &cluster.Envelope{Pid: -1, Msg: b}
			}
		} else if r.mode == "F" {

			// Follower "F" Mode

			select {
			case e := <-r.peerObject.Inbox():
				err := json.Unmarshal(e.Msg.([]byte), &msg)
				if err != nil {
					panic(err)
				}

				if msg.MsgType == 1 {
					if msg.CallTo == 1 {
						t, vote := r.RequestVote(msg.Term, msg.Id)
						b, err := json.Marshal(RaftMessage{2, t, vote, r.peerObject.Pid(), 1})
						if err != nil {
							panic(err)
						}
						r.peerObject.Outbox() <- &cluster.Envelope{Pid: msg.Id, Msg: b}
					} else if msg.CallTo == 2 {
						t, taskDone := r.AppendEntries(msg.Term, msg.Id)
						//fmt.Printf("Term changed for peer %d to %d\n", r.peerObject.Pid(), r.currentTerm)
						b, err := json.Marshal(RaftMessage{2, t, taskDone, r.peerObject.Pid(), 2})
						if err != nil {
							panic(err)
						}
						r.peerObject.Outbox() <- &cluster.Envelope{Pid: msg.Id, Msg: b}
					}
				}
			case <-time.After(time.Duration(rand.Intn(UpperElectionTO-LowerElectionTO)+LowerElectionTO) * time.Millisecond):
				r.mode = "C"
				r.currentTerm++
				r.totalVotes = 1
				r.votesFrom = string(r.peerObject.Pid())
				if DEBUG == 1 {
					fmt.Printf("Peer %d turning to candidate for term %d\n", r.peerObject.Pid(), r.currentTerm)
				}

				b, err := json.Marshal(RaftMessage{1, r.currentTerm, false, r.peerObject.Pid(), 1})
				if err != nil {
					panic(err)
				}
				r.peerObject.Outbox() <- &cluster.Envelope{Pid: -1, Msg: b}
			}
		}
	}
}

// Create and return a new raft object
func AddRaftPeer(id int, config string) Raft {

	type ConfigData struct {
		HeartBeat       int    //in ms
		LowerElectionTO int    //in ms
		UpperElectionTO int    //in ms
		NumPeers        int    //total number of peers
		ClusterConfig   string //filename of the .json with cluster info
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

	//fmt.Printf("%d %d %d %d %s\n", c.HeartBeat, c.LowerElectionTO, c.UpperElectionTO, c.NumPeers, c.ClusterConfig)
	HeartBeat = c.HeartBeat
	UpperElectionTO = c.UpperElectionTO
	LowerElectionTO = c.LowerElectionTO
	clusterObject := cluster.AddPeer(id, c.ClusterConfig)

	seed := int64(100 + id)
	rand.Seed(seed)

	Me := new(RaftBody)
	Me.mode = "F"
	Me.peerObject = clusterObject
	Me.NumServers = c.NumPeers
	Me.currentTerm = 0
	Me.votedFor = 0

	go Runnable(Me, c.HeartBeat, c.LowerElectionTO, c.UpperElectionTO)

	return Me
}

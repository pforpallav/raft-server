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
	MsgType int // 1 for Request, 2 for Response
	Term    int
	Result2 bool
	Id      int
	CallTo  int // 1 for RequestVote, 2 for AppendEntries
}

type LeaderInfo struct {
	LeaderId int
	Term int
	MajorityFrom string
}

type Raft interface {
	// The current term for this peer
	Term() int

	// The current leader according to this peer
	isLeader() bool

	// Pause/Unpause - drop messages recieved and to be sent
	Pause() bool
	Unpause() bool
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

var LeaderChan chan *LeaderInfo

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

func (r *RaftBody) RequestVote(term int, candidateId int /*, lastLogIndex int, lastLogTerm int*/) (int, bool) {
	if (r.mode == "F") && (r.currentTerm < term) {
		r.mode = "F"
		r.currentTerm = term
		r.votedFor = candidateId
		fmt.Printf("%d voting for %d for %d term\n", r.peerObject.Pid(), candidateId, term)
		return r.currentTerm, true
	} else {
		return r.currentTerm, false
	}
}

func (r *RaftBody) AppendEntries(term int, leaderId int /*, prevLogIndex int, entries []interface{}, leaderCommit int*/) (int, bool) {
	if r.currentTerm < term {
		r.mode = "F"
		fmt.Printf("Term changed for peer %d from %d to %d\n", r.peerObject.Pid(), r.currentTerm, term)
		r.currentTerm = term
	}
	return r.currentTerm, true
}


func Heartbeat(r *RaftBody, HeartBeat int) {
		for {

			if(r.mode != "L"){
				fmt.Printf("Heart break for %d\n", r.peerObject.Pid())
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


func Runnable(r *RaftBody , HeartBeat int, LowerElectionTO int, UpperElectionTO int) int {

	totalVotes := 0
	votesFrom := ""
	var msg RaftMessage

	seed := int64(100 + r.peerObject.Pid())
	rand.Seed(seed)

	for {
		if r.mode == "L" {
			select {
			case e := <-r.peerObject.Inbox():

				err := json.Unmarshal(e.Msg.([]byte), &msg)
				if err != nil {
					panic(err)
				}

				if msg.Term > r.currentTerm {
					fmt.Printf("WTFFFF from %d to %d\n", msg.Id, r.peerObject.Pid())

					r.mode = "F"
					totalVotes = 0
					votesFrom = ""
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

			select {
			case e := <-r.peerObject.Inbox():
				err := json.Unmarshal(e.Msg.([]byte), &msg)
				if err != nil {
					panic(err)
				}

				if msg.Term > r.currentTerm {
					r.mode = "F"
					totalVotes = 0
					votesFrom = ""
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
							totalVotes++
							votesFrom += " " + string(msg.Id)
							if totalVotes > r.NumServers/2 && r.mode == "C" {
								r.mode = "L"
								go Heartbeat(r, HeartBeat)
								//fmt.Printf("%d \t %d \t %d \t %q\n", r.peerObject.Pid(), r.currentTerm, totalVotes, votesFrom)
								LeaderChan <- &LeaderInfo{r.peerObject.Pid(), r.currentTerm, votesFrom}
							}
						}
					} else if msg.CallTo == 2 {
						if msg.Term > r.currentTerm {
							r.mode = "F"
							totalVotes = 0
							votesFrom = ""
						}
					}
				}

			case <-time.After(time.Duration(rand.Intn(UpperElectionTO-LowerElectionTO) + LowerElectionTO) * time.Millisecond):
				fmt.Printf("Peer %d turning to candidate for term %d\n", r.peerObject.Pid(), r.currentTerm)
				r.currentTerm++
				totalVotes = 1
				votesFrom = string(r.peerObject.Pid())
				b, err := json.Marshal(RaftMessage{1, r.currentTerm, false, r.peerObject.Pid(), 1})
				if err != nil {
					panic(err)
				}
				r.peerObject.Outbox() <- &cluster.Envelope{Pid: -1, Msg: b}
			}
		} else if r.mode == "F" {
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

			case <-time.After(time.Duration(rand.Intn(UpperElectionTO-LowerElectionTO) + LowerElectionTO) * time.Millisecond):
				r.mode = "C"
				r.currentTerm = r.currentTerm + 1
				totalVotes = 1
				votesFrom = string(r.peerObject.Pid())
				fmt.Printf("Peer %d turning to candidate for term %d\n", r.peerObject.Pid(), r.currentTerm)
				
				b, err := json.Marshal(RaftMessage{1, r.currentTerm, false, r.peerObject.Pid(), 1})
				if err != nil {
					panic(err)
				}
				r.peerObject.Outbox() <- &cluster.Envelope{Pid: -1, Msg: b}
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

	Me := new(RaftBody)
	Me.mode = "F"
	Me.peerObject = clusterObject
	Me.NumServers = c.NumPeers
	Me.currentTerm = 0
	Me.votedFor = 0

	go Runnable(Me, c.HeartBeat, c.LowerElectionTO, c.UpperElectionTO)

	return Me
}

package raft

import (
	//"fmt"
	"testing"
	"time"
)

const (
	NUMPEERS = 5
	MAINLOOP = 1000
)

func TestRaft(t *testing.T){
	var PeerArray [NUMPEERS]Raft
	println("Leader\tTerm\tVotes\tFrom")
	for i := 1; i <= NUMPEERS; i++ {
		PeerArray[i-1] = AddRaftPeer(i, "raftConfig.json")
	}
	<-time.After(5 * time.Second)
}
package raft

import (
	"fmt"
	"testing"
	"time"
)

const (
	NUMPEERS = 5
	MAINLOOP = 1000
)

func TestRaft(t *testing.T){
	var PeerArray [NUMPEERS]Raft
	println("Leader\tTerm\tVotesFrom")
	for i := 1; i <= NUMPEERS; i++ {
		PeerArray[i-1] = AddRaftPeer(i, "raftConfig.json")
	}
	
	select{
		case l := <- LeaderChan:
			fmt.Printf("%d\t%d\t%q\n", l.LeaderId, l.Term, l.MajorityFrom)
		case <- time.After(2 * time.Second):
			t.Error("No leader elected!")
	}

	for i := 1; i <= NUMPEERS; i++ {
		if PeerArray[i-1].isLeader() {
			PeerArray[i-1].Pause()
			PeerArray[i%5].Pause()
			println("Two peers ", i-1, " & ", i%5, " cut-off!")
		}
	}

	select{
		case l := <- LeaderChan:
			fmt.Printf("%d\t%d\t%q\n", l.LeaderId, l.Term, l.MajorityFrom)
		case <- time.After(2 * time.Second):
			t.Error("No leader elected!")
	}
}
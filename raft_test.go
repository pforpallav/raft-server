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

func TestRaft(t *testing.T) {
	var PeerArray [NUMPEERS]Raft
	var i int

	println("Leader\tTerm\tVotesFrom")

	for i = 1; i <= NUMPEERS; i++ {
		PeerArray[i-1] = AddRaftPeer(i, "raftConfig.json")
	}
    
    println("TESTING BEGINS\n\n")
	println("\n---------------------Peers Started!-----------------------")
	println("-------------------Expecting Election---------------------\n")

	select {
	case l := <-LeaderChan:
		fmt.Printf("%d\t%d\t%q\n", l.LeaderId, l.Term, l.MajorityFrom)
	case <-time.After(2 * time.Second):
		t.Error("No leader elected!")
	}
	select {
	case l := <-LeaderChan:
		fmt.Printf("%d\t%d\t%q\n", l.LeaderId, l.Term, l.MajorityFrom)
		t.Error("More than one leaders!")
	case <-time.After(2 * time.Second):
	}

	for i = 1; i <= NUMPEERS; i++ {
		if PeerArray[i-1].isLeader() {
			PeerArray[i-1].Pause()
			PeerArray[i%5].Pause()
			println("\n----------Two peers ", i, "(Leader) & ", i%5+1, " cut-off!------------")
			println("-----------------Expecting re-election--------------------\n")
			break
		}
	}

	select {
	case l := <-LeaderChan:
		fmt.Printf("%d\t%d\t%q\n", l.LeaderId, l.Term, l.MajorityFrom)
	case <-time.After(2 * time.Second):
		t.Error("No leader elected!")
	}
	select {
	case l := <-LeaderChan:
		fmt.Printf("%d\t%d\t%q\n", l.LeaderId, l.Term, l.MajorityFrom)
		t.Error("More than one leaders!")
	case <-time.After(2 * time.Second):
	}

	PeerArray[i-1].Unpause()
	PeerArray[i%5].Unpause()
	println("\n----------Two peers ", i, "(Ex-Leader) & ", i%5+1, " back!------------")
	println("-----------------Expecting re-election--------------------\n")

	select {
	case l := <-LeaderChan:
		fmt.Printf("%d\t%d\t%q\n", l.LeaderId, l.Term, l.MajorityFrom)
	case <-time.After(2 * time.Second):
		t.Error("No leader elected!")
	}
	select {
	case l := <-LeaderChan:
		fmt.Printf("%d\t%d\t%q\n", l.LeaderId, l.Term, l.MajorityFrom)
		t.Error("More than one leaders!")
	case <-time.After(2 * time.Second):
	}

	for i = 1; i <= NUMPEERS; i++ {
		if PeerArray[i-1].isLeader() {
			PeerArray[i-1].Pause()
			PeerArray[i%5].Pause()
			PeerArray[(i+1)%5].Pause()
			println("\n-------Two peers ", i, "(Leader), ", i%5+1, " & ", (i+1)%5+1, " cut-off!----------")
			println("-----------------Expecting NO election--------------------\n")
			break
		}
	}

	select {
	case l := <-LeaderChan:
		fmt.Printf("%d\t%d\t%q\n", l.LeaderId, l.Term, l.MajorityFrom)
		t.Error("Majority failure unsuccessful!")
	case <-time.After(2 * time.Second):
	}

	PeerArray[i-1].Unpause()
	PeerArray[i%5].Unpause()
	PeerArray[(i+1)%5].Unpause()
	println("\n---------Two peers ", i, "(Leader), ", i%5+1, " & ", (i+1)%5+1, " back!-----------")
	println("-----------------Expecting re-election--------------------\n")

	select {
	case l := <-LeaderChan:
		fmt.Printf("%d\t%d\t%q\n", l.LeaderId, l.Term, l.MajorityFrom)
	case <-time.After(2 * time.Second):
		t.Error("No leader elected!")
	}
	select {
	case l := <-LeaderChan:
		fmt.Printf("%d\t%d\t%q\n", l.LeaderId, l.Term, l.MajorityFrom)
		t.Error("More than one leaders!")
	case <-time.After(2 * time.Second):
	}

	print("\nTest Result: ")

}

package sapient_test

import (
	"testing"
	"time"

	"sapient/pkg/sapient"
	pb "sapient/pkg/sapientpb"
)

func TestMultiSensorRouting(t *testing.T) {
	peer, err := sapient.Dial(peerAddr)
	if err != nil {
		t.Fatalf("dial peer: %v", err)
	}
	defer peer.Close()

	child1, child1ID := dialChild(t)
	defer child1.Close()

	child2, _ := dialChild(t)
	defer child2.Close()

	// Drain both registrations on peer side
	for i := 0; i < 2; {
		msg, _ := peer.Recv()
		if msg != nil && sapient.ContentType(msg) == "registration" {
			i++
		}
	}

	// Task to child1 only
	peerID := sapient.NewUUID()
	task := sapient.NewTask(pb.Task_CONTROL_START).Request("Start").Build()
	peer.SendTask(peerID, child1ID, task)

	msg := recvExpect(t, child1, "task")
	t.Logf("child1 got task: %s", msg.GetTask().GetTaskId())

	// child2 should NOT receive anything
	done := make(chan bool, 1)
	go func() {
		_, err := child2.Recv()
		if err == nil {
			done <- true
		}
	}()
	select {
	case <-done:
		t.Fatal("child2 received a message it shouldn't have")
	case <-time.After(500 * time.Millisecond):
		t.Log("child2 correctly received nothing")
	}
}

func TestStatusUnchangedValidation(t *testing.T) {
	child, childID := dialChild(t)
	defer child.Close()

	child.SendStatus(childID, sapient.NewStatusReport(pb.StatusReport_SYSTEM_OK, pb.StatusReport_INFO_UNCHANGED, "Surveillance").Build())
	msg := recvExpect(t, child, "error")
	t.Logf("Apex correctly rejected: %v", msg.GetError().GetErrorMessage())
}

func TestReRegistration(t *testing.T) {
	child, childID := dialChild(t)
	defer child.Close()

	// Send second registration on same connection
	reg2 := testRegistration()
	reg2.Name = strp("Updated Radar")
	msg := sapient.Msg(childID)
	msg.Content = &pb.SapientMessage_Registration{Registration: reg2}
	child.Send(msg)

	reply, err := child.Recv()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	if sapient.ContentType(reply) == "error" {
		t.Fatalf("rejected: %v", reply.GetError().GetErrorMessage())
	}
	t.Logf("re-registration accepted")
}

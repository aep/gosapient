package sapient_test

import (
	"testing"
	"time"

	"sapient/pkg/sapient"
	pb "sapient/pkg/sapientpb"
)

// TestMultiSensorRouting registers two sensors, sends a Task to one,
// and verifies only that sensor receives it.
func TestMultiSensorRouting(t *testing.T) {
	peer, err := sapient.DialPeer(peerAddr, nil)
	if err != nil {
		t.Fatalf("DialPeer: %v", err)
	}
	defer peer.Close()

	// Register two sensors
	child1, err := sapient.DialChild(childAddr, testRegistration())
	if err != nil {
		t.Fatalf("DialChild 1: %v", err)
	}
	defer child1.Close()

	reg2 := testRegistration()
	reg2.Name = strp("Test Radar 2")
	reg2.ShortName = strp("TEST2")
	child2, err := sapient.DialChild(childAddr, reg2)
	if err != nil {
		t.Fatalf("DialChild 2: %v", err)
	}
	defer child2.Close()

	// Drain peer registrations
	drainRegistrations(t, peer, 2)

	// Send Task to child1 only
	task := sapient.NewTask(pb.Task_CONTROL_START).
		Request("Start").
		Build()

	if err := peer.SendTask(child1.NodeID, task); err != nil {
		t.Fatalf("SendTask: %v", err)
	}

	// child1 should receive the task
	msg1 := recvExpect(t, child1.Conn, "task")
	t.Logf("child1 got task: %s", msg1.GetTask().GetTaskId())

	// child2 should NOT receive anything
	done := make(chan bool, 1)
	go func() {
		_, err := child2.Conn.Recv()
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

// TestStatusUnchangedValidation verifies that Apex rejects INFO_UNCHANGED
// before any INFO_NEW has been sent.
func TestStatusUnchangedValidation(t *testing.T) {
	child, err := sapient.DialChild(childAddr, testRegistration())
	if err != nil {
		t.Fatalf("DialChild: %v", err)
	}
	defer child.Close()

	// Send UNCHANGED without prior NEW — Apex should return an error
	bad := sapient.NewStatusReport(
		pb.StatusReport_SYSTEM_OK,
		pb.StatusReport_INFO_UNCHANGED,
		"Surveillance",
	).Build()

	if err := child.SendStatus(bad); err != nil {
		t.Fatalf("SendStatus: %v", err)
	}

	// Expect an error reply
	reply, err := child.Conn.Recv()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}

	ct := sapient.ContentType(reply)
	if ct != "error" {
		t.Fatalf("expected error, got %s", ct)
	}
	t.Logf("Apex correctly rejected: %v", reply.GetError().GetErrorMessage())
}

// TestReRegistration verifies that a sensor can re-register with the same node_id.
func TestReRegistration(t *testing.T) {
	child, err := sapient.DialChild(childAddr, testRegistration())
	if err != nil {
		t.Fatalf("DialChild: %v", err)
	}
	defer child.Close()

	// Send a second registration on the same connection
	reg2 := testRegistration()
	reg2.Name = strp("Updated Radar")

	msg := sapient.Msg(child.NodeID)
	msg.Content = &pb.SapientMessage_Registration{Registration: reg2}
	if err := child.Conn.Send(msg); err != nil {
		t.Fatalf("send re-registration: %v", err)
	}

	// Should get another RegistrationAck
	reply, err := child.Conn.Recv()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}

	ct := sapient.ContentType(reply)
	if ct == "error" {
		t.Fatalf("Apex rejected re-registration: %v", reply.GetError().GetErrorMessage())
	}
	if ct != "registration_ack" {
		t.Fatalf("expected registration_ack, got %s", ct)
	}
	t.Logf("re-registration accepted")
}

func drainRegistrations(t *testing.T, peer *sapient.Peer, minCount int) {
	t.Helper()
	count := 0
	for {
		done := make(chan *pb.SapientMessage, 1)
		go func() {
			msg, err := peer.Conn.Recv()
			if err != nil {
				return
			}
			done <- msg
		}()

		select {
		case msg := <-done:
			if sapient.ContentType(msg) == "registration" {
				count++
				t.Logf("drained registration %d from %s", count, msg.GetNodeId())
				if count >= minCount {
					return
				}
			}
		case <-time.After(3 * time.Second):
			if count >= minCount {
				return
			}
			t.Fatalf("timeout draining registrations (got %d, need %d)", count, minCount)
		}
	}
}

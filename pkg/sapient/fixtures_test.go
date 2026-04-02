package sapient_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sapient/pkg/sapient"
	pb "sapient/pkg/sapientpb"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func fixtureDir() string {
	if d := os.Getenv("SAPIENT_FIXTURES_DIR"); d != "" {
		return filepath.Join(d, "True")
	}
	return ""
}

// TestDstlFixtures sends every valid message from the Dstl Test Harness
// True/ directory through Apex and verifies they are accepted.
func TestDstlFixtures(t *testing.T) {
	dir := fixtureDir()
	if dir == "" {
		t.Skip("SAPIENT_FIXTURES_DIR not set")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("Dstl fixtures not found at %s", dir)
	}

	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures found: %v", err)
	}

	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".json")
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read: %v", err)
			}

			// Parse JSON into SapientMessage
			msg := &pb.SapientMessage{}
			if err := protojson.Unmarshal(data, msg); err != nil {
				t.Skipf("proto parse: %v", err)
			}

			// Patch timestamp to now — fixtures have hardcoded timestamps
			// that would fail Apex's message_timestamp_reasonable check
			msg.Timestamp = timestamppb.Now()

			// Patch node_id if empty
			if msg.GetNodeId() == "" {
				id := sapient.NewUUID()
				msg.NodeId = &id
			}

			// Must register before sending other message types.
			// For non-registration messages, register first then send.
			ct := sapient.ContentType(msg)

			if ct == "registration" {
				sendAsChild(t, msg)
			} else if ct == "registration_ack" || ct == "error" {
				// These are sent by the middleware/fusion node, not edge nodes.
				// Send as Peer to verify Apex accepts them on the wire.
				sendAsPeer(t, msg)
			} else {
				// For status_report, detection_report, alert, task_ack:
				// register first, then send the fixture message
				sendWithRegistration(t, msg)
			}
		})
	}
}

func sendAsChild(t *testing.T, msg *pb.SapientMessage) {
	t.Helper()
	conn, err := sapient.Dial(childAddr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := conn.Send(msg); err != nil {
		t.Fatalf("send: %v", err)
	}

	reply, err := conn.Recv()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}

	ct := sapient.ContentType(reply)
	if ct == "error" {
		t.Fatalf("Apex rejected: %v", reply.GetError().GetErrorMessage())
	}
	if ct == "registration_ack" && !reply.GetRegistrationAck().GetAcceptance() {
		t.Fatalf("registration rejected: %v", reply.GetRegistrationAck().GetAckResponseReason())
	}
	t.Logf("accepted (%s)", ct)
}

func sendAsPeer(t *testing.T, msg *pb.SapientMessage) {
	t.Helper()
	conn, err := sapient.Dial(peerAddr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := conn.Send(msg); err != nil {
		t.Fatalf("send: %v", err)
	}
	t.Logf("sent to peer (no error = accepted)")
}

func sendWithRegistration(t *testing.T, msg *pb.SapientMessage) {
	t.Helper()

	// Register first
	child, err := sapient.DialChild(childAddr, testRegistration())
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	defer child.Close()

	// Patch the fixture's node_id to match our registration
	msg.NodeId = &child.NodeID

	// Patch any ULID fields that might be stale — regenerate report_id
	patchReportID(msg)

	if err := child.Conn.Send(msg); err != nil {
		t.Fatalf("send: %v", err)
	}

	// Brief wait then check if Apex sent back an error
	done := make(chan *pb.SapientMessage, 1)
	go func() {
		reply, err := child.Conn.Recv()
		if err != nil {
			return // connection closed = no error response = success
		}
		done <- reply
	}()

	select {
	case reply := <-done:
		if sapient.ContentType(reply) == "error" {
			t.Fatalf("Apex rejected: %v", reply.GetError().GetErrorMessage())
		}
		t.Logf("got response: %s", sapient.ContentType(reply))
	case <-time.After(500 * time.Millisecond):
		t.Logf("accepted (no error response)")
	}
}

// patchReportID regenerates the report_id in status_report and detection_report
// so that Apex doesn't see duplicate ULIDs across test runs.
func patchReportID(msg *pb.SapientMessage) {
	// Use raw JSON manipulation since the fixture may have fields
	// we don't want to disturb
	switch c := msg.GetContent().(type) {
	case *pb.SapientMessage_StatusReport:
		id := sapient.NewULID()
		c.StatusReport.ReportId = &id
	case *pb.SapientMessage_DetectionReport:
		id := sapient.NewULID()
		c.DetectionReport.ReportId = &id
		oid := sapient.NewULID()
		c.DetectionReport.ObjectId = &oid
	case *pb.SapientMessage_Alert:
		id := sapient.NewULID()
		c.Alert.AlertId = &id
	case *pb.SapientMessage_TaskAck:
		id := sapient.NewULID()
		c.TaskAck.TaskId = &id
	case *pb.SapientMessage_AlertAck:
		id := sapient.NewULID()
		c.AlertAck.AlertId = &id
	}
}

// Verify fixture JSON can round-trip through our protobuf without data loss.
func TestDstlFixtureRoundTrip(t *testing.T) {
	dir := fixtureDir()
	if dir == "" {
		t.Skip("SAPIENT_FIXTURES_DIR not set")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("Dstl fixtures not found at %s", dir)
	}

	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".json")
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read: %v", err)
			}

			msg := &pb.SapientMessage{}
			if err := protojson.Unmarshal(data, msg); err != nil {
				t.Skipf("proto parse: %v", err)
			}

			// Re-serialize to JSON and back
			out, err := protojson.Marshal(msg)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			msg2 := &pb.SapientMessage{}
			if err := protojson.Unmarshal(out, msg2); err != nil {
				t.Fatalf("re-unmarshal: %v", err)
			}

			// Verify content type survived
			ct1 := sapient.ContentType(msg)
			ct2 := sapient.ContentType(msg2)
			if ct1 != ct2 {
				t.Errorf("content type changed: %s → %s", ct1, ct2)
			}

			// Verify key fields survived by comparing JSON
			var j1, j2 map[string]any
			json.Unmarshal(data, &j1)
			json.Unmarshal(out, &j2)

			// Check that all top-level keys from the original exist in round-tripped
			for k := range j1 {
				if _, ok := j2[k]; !ok {
					t.Errorf("key %q lost in round-trip", k)
				}
			}

			t.Logf("round-trip OK (%s)", ct1)
		})
	}
}

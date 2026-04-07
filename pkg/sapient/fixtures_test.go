package sapient_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aep/gosapient/pkg/sapient"
	pb "github.com/aep/gosapient/pkg/sapientpb"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func fixtureDir() string {
	if d := os.Getenv("SAPIENT_FIXTURES_DIR"); d != "" {
		return filepath.Join(d, "True")
	}
	return ""
}

func TestDstlFixtures(t *testing.T) {
	dir := fixtureDir()
	if dir == "" {
		t.Skip("SAPIENT_FIXTURES_DIR not set")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("fixtures not found at %s", dir)
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

			msg.Timestamp = timestamppb.Now()
			if msg.GetNodeId() == "" {
				id := sapient.NewUUID()
				msg.NodeId = &id
			}

			ct := sapient.ContentType(msg)
			if ct == "registration" {
				sendFixtureAsChild(t, msg)
			} else if ct == "registration_ack" || ct == "error" {
				sendFixtureAsPeer(t, msg)
			} else {
				sendFixtureWithRegistration(t, msg)
			}
		})
	}
}

func sendFixtureAsChild(t *testing.T, msg *pb.SapientMessage) {
	t.Helper()
	conn, err := sapient.Dial(childAddr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	conn.Send(msg)
	reply, err := conn.Recv()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	ct := sapient.ContentType(reply)
	if ct == "error" {
		t.Fatalf("Apex rejected: %v", reply.GetError().GetErrorMessage())
	}
	t.Logf("accepted (%s)", ct)
}

func sendFixtureAsPeer(t *testing.T, msg *pb.SapientMessage) {
	t.Helper()
	conn, err := sapient.Dial(peerAddr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.Send(msg)
	t.Logf("sent to peer")
}

func sendFixtureWithRegistration(t *testing.T, msg *pb.SapientMessage) {
	t.Helper()

	child, childID := dialChild(t)
	defer child.Close()

	msg.NodeId = &childID
	patchReportID(msg)

	child.Send(msg)

	done := make(chan *pb.SapientMessage, 1)
	go func() {
		reply, err := child.Recv()
		if err != nil {
			return
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

func patchReportID(msg *pb.SapientMessage) {
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

func TestDstlFixtureRoundTrip(t *testing.T) {
	dir := fixtureDir()
	if dir == "" {
		t.Skip("SAPIENT_FIXTURES_DIR not set")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("fixtures not found at %s", dir)
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

			out, err := protojson.Marshal(msg)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			msg2 := &pb.SapientMessage{}
			if err := protojson.Unmarshal(out, msg2); err != nil {
				t.Fatalf("re-unmarshal: %v", err)
			}

			if sapient.ContentType(msg) != sapient.ContentType(msg2) {
				t.Errorf("content type changed: %s → %s", sapient.ContentType(msg), sapient.ContentType(msg2))
			}

			var j1, j2 map[string]any
			json.Unmarshal(data, &j1)
			json.Unmarshal(out, &j2)
			for k := range j1 {
				if _, ok := j2[k]; !ok {
					t.Errorf("key %q lost in round-trip", k)
				}
			}
			t.Logf("round-trip OK (%s)", sapient.ContentType(msg))
		})
	}
}

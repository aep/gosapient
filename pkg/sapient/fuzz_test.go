package sapient_test

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/aep/gosapient/pkg/sapient"
	pb "github.com/aep/gosapient/pkg/sapientpb"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// FuzzRecv feeds random bytes into the transport layer's Recv.
// Verifies that no input causes a panic.
func FuzzRecv(f *testing.F) {
	// Seed with a valid message
	msg := sapient.Msg(sapient.NewUUID())
	msg.Content = &pb.SapientMessage_Error{Error: &pb.Error{
		Packet:       []byte("test"),
		ErrorMessage: []string{"test"},
	}}
	valid, _ := proto.Marshal(msg)
	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, uint32(len(valid)))
	f.Add(append(header, valid...))

	// Seed with truncated header
	f.Add([]byte{0x05})
	// Seed with zero-length message
	f.Add([]byte{0x00, 0x00, 0x00, 0x00})
	// Seed with garbage
	f.Add([]byte{0xff, 0xff, 0xff, 0xff, 0x01, 0x02})

	f.Fuzz(func(t *testing.T, data []byte) {
		server, client := net.Pipe()
		defer server.Close()
		defer client.Close()

		conn := sapient.NewConn(client)

		go func() {
			server.Write(data)
			server.Close()
		}()

		client.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		// Must not panic — errors are fine
		conn.Recv()
	})
}

// FuzzUnmarshalSapientMessage feeds random bytes directly into protobuf
// deserialization. Verifies that ContentType never panics on the result.
func FuzzUnmarshalSapientMessage(f *testing.F) {
	// Seed with valid protobuf bytes for each message type
	for _, msg := range seedMessages() {
		data, _ := proto.Marshal(msg)
		f.Add(data)
	}
	f.Add([]byte{})
	f.Add([]byte{0x08, 0x01}) // varint field

	f.Fuzz(func(t *testing.T, data []byte) {
		msg := &pb.SapientMessage{}
		if err := proto.Unmarshal(data, msg); err != nil {
			return // invalid proto is fine
		}
		// Must not panic
		sapient.ContentType(msg)
		msg.GetNodeId()
		msg.GetDestinationId()
		msg.GetTimestamp()
	})
}

// FuzzUnmarshalJSON feeds random bytes into protobuf JSON deserialization.
func FuzzUnmarshalJSON(f *testing.F) {
	f.Add([]byte(`{"nodeId":"abc","timestamp":"2025-01-01T00:00:00Z","registration":{}}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`{"statusReport":{"system":"SYSTEM_OK"}}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		msg := &pb.SapientMessage{}
		// Must not panic
		protojson.Unmarshal(data, msg)
	})
}

func seedMessages() []*pb.SapientMessage {
	nodeID := sapient.NewUUID()
	ulid := sapient.NewULID()
	acc := true
	return []*pb.SapientMessage{
		{NodeId: &nodeID, Content: &pb.SapientMessage_Registration{Registration: testRegistration()}},
		{NodeId: &nodeID, Content: &pb.SapientMessage_RegistrationAck{RegistrationAck: &pb.RegistrationAck{Acceptance: &acc}}},
		{NodeId: &nodeID, Content: &pb.SapientMessage_StatusReport{StatusReport: sapient.NewStatusReport(pb.StatusReport_SYSTEM_OK, pb.StatusReport_INFO_NEW, "test").Build()}},
		{NodeId: &nodeID, Content: &pb.SapientMessage_DetectionReport{DetectionReport: sapient.NewDetection(ulid).Location(51.5, -1.2, 30).Build()}},
		{NodeId: &nodeID, Content: &pb.SapientMessage_Task{Task: sapient.NewTask(pb.Task_CONTROL_START).Request("Start").Build()}},
		{NodeId: &nodeID, Content: &pb.SapientMessage_TaskAck{TaskAck: sapient.NewTaskAck(ulid, pb.TaskAck_TASK_STATUS_ACCEPTED)}},
		{NodeId: &nodeID, Content: &pb.SapientMessage_Alert{Alert: sapient.NewAlert(pb.Alert_ALERT_TYPE_WARNING, pb.Alert_ALERT_STATUS_ACTIVE, pb.Alert_DISCRETE_PRIORITY_HIGH, "test")}},
		{NodeId: &nodeID, Content: &pb.SapientMessage_AlertAck{AlertAck: sapient.NewAlertAck(ulid, pb.AlertAck_ALERT_ACK_STATUS_ACCEPTED)}},
		{NodeId: &nodeID, Content: &pb.SapientMessage_Error{Error: &pb.Error{Packet: []byte("test"), ErrorMessage: []string{"err"}}}},
	}
}

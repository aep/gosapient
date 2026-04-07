package sapient

import (
	v1 "github.com/aep/gosapient/pkg/sapientpb/v1"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// V1/V2 wire compatibility summary:
//
// Receiving: Recv() handles both v1 and v2 — protobuf field numbers are
// identical for all message content, so v1 bytes deserialize into v2 structs.
//
// Sending: Most messages work both ways. Three messages have fields that
// moved to new field numbers in v2, so a v1 client won't see them:
//
//   RegistrationAck.ack_response_reason  (field 2 → 3)
//   TaskAck.reason                       (field 3 → 5)
//   AlertAck.alert_ack_status            (field 2 → 5)
//   AlertAck.reason                      (field 3 → 4)
//
// Use the V1 send functions below when talking to nodes with
// icd_version "7.0" or "BSI Flex 335 v1.0".
// All other messages (Task, StatusReport, DetectionReport, Alert, Error)
// are wire-identical and don't need special handling.

// RecvV1 reads a message as v1. Only needed if you specifically need v1
// struct access. Recv() works for both versions in most cases.
func (c *Conn) RecvV1() (*v1.SapientMessage, error) {
	data, err := c.RecvRaw()
	if err != nil {
		return nil, err
	}
	msg := &v1.SapientMessage{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// SendV1 sends an arbitrary v1 SapientMessage.
func (c *Conn) SendV1(msg *v1.SapientMessage) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	return c.SendRaw(data)
}

// AckV1 sends a v1-compatible RegistrationAck.
func AckV1(conn *Conn, middlewareID, destinationID string, accept bool, reason string) error {
	msg := &v1.SapientMessage{
		Timestamp:     timestamppb.Now(),
		NodeId:        middlewareID,
		DestinationId: &destinationID,
		Content: &v1.SapientMessage_RegistrationAck{
			RegistrationAck: &v1.RegistrationAck{
				Acceptance:        accept,
				AckResponseReason: &reason,
			},
		},
	}
	return conn.SendV1(msg)
}

// SendTaskAckV1 sends a v1-compatible TaskAck.
func (c *Conn) SendTaskAckV1(nodeID string, taskID string, status v1.TaskAck_TaskStatus, reason string) error {
	msg := &v1.SapientMessage{
		Timestamp: timestamppb.Now(),
		NodeId:    nodeID,
		Content: &v1.SapientMessage_TaskAck{
			TaskAck: &v1.TaskAck{
				TaskId:     taskID,
				TaskStatus: status,
				Reason:     &reason,
			},
		},
	}
	return c.SendV1(msg)
}

// SendAlertAckV1 sends a v1-compatible AlertAck.
func (c *Conn) SendAlertAckV1(nodeID, destinationID string, alertID string, status v1.AlertAck_AlertStatus, reason string) error {
	msg := &v1.SapientMessage{
		Timestamp:     timestamppb.Now(),
		NodeId:        nodeID,
		DestinationId: &destinationID,
		Content: &v1.SapientMessage_AlertAck{
			AlertAck: &v1.AlertAck{
				AlertId:     alertID,
				AlertStatus: status,
				Reason:      &reason,
			},
		},
	}
	return c.SendV1(msg)
}

// IsV1 returns true if the registration's icd_version indicates v1 or v7.
func IsV1(icdVersion string) bool {
	return icdVersion != "" && icdVersion != "BSI Flex 335 v2.0"
}

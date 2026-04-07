package sapient

import (
	"fmt"

	pb "sapient/pkg/sapientpb"
)

// Register sends a Registration on conn and waits for a RegistrationAck.
// Returns the assigned node_id on success.
func Register(conn *Conn, reg *pb.Registration) (nodeID string, err error) {
	nodeID = NewUUID()

	msg := Msg(nodeID)
	msg.Content = &pb.SapientMessage_Registration{Registration: reg}

	if err := conn.Send(msg); err != nil {
		return "", fmt.Errorf("send registration: %w", err)
	}

	reply, err := conn.Recv()
	if err != nil {
		return "", fmt.Errorf("recv registration ack: %w", err)
	}

	switch c := reply.GetContent().(type) {
	case *pb.SapientMessage_RegistrationAck:
		if !c.RegistrationAck.GetAcceptance() {
			return "", fmt.Errorf("registration rejected: %v", c.RegistrationAck.GetAckResponseReason())
		}
		return nodeID, nil
	case *pb.SapientMessage_Error:
		return "", fmt.Errorf("registration error: %v", c.Error.GetErrorMessage())
	default:
		return "", fmt.Errorf("unexpected response: %s", ContentType(reply))
	}
}

// Ack sends a RegistrationAck to a node that just registered.
func Ack(conn *Conn, middlewareID, destinationID string, accept bool, reasons ...string) error {
	msg := Msg(middlewareID)
	msg.DestinationId = &destinationID
	msg.Content = &pb.SapientMessage_RegistrationAck{
		RegistrationAck: &pb.RegistrationAck{
			Acceptance:       &accept,
			AckResponseReason: reasons,
		},
	}
	return conn.Send(msg)
}

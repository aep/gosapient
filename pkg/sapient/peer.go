package sapient

import (
	"fmt"

	pb "sapient/pkg/sapientpb"
)

// Peer represents a SAPIENT fusion node / C2 connection.
// It connects to the middleware as a Peer and receives sensor
// registrations, status reports, and detections, while sending tasks.
type Peer struct {
	Conn   *Conn
	NodeID string
}

// DialPeer connects to a SAPIENT middleware Peer port.
// Optionally sends a Registration for the fusion node itself.
func DialPeer(addr string, reg *pb.Registration) (*Peer, error) {
	conn, err := Dial(addr)
	if err != nil {
		return nil, err
	}

	nodeID := NewUUID()
	peer := &Peer{Conn: conn, NodeID: nodeID}

	if reg != nil {
		msg := Msg(nodeID)
		msg.Content = &pb.SapientMessage_Registration{Registration: reg}
		if err := conn.Send(msg); err != nil {
			conn.Close()
			return nil, fmt.Errorf("sapient peer send registration: %w", err)
		}

		reply, err := conn.Recv()
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("sapient peer recv ack: %w", err)
		}

		switch c := reply.GetContent().(type) {
		case *pb.SapientMessage_RegistrationAck:
			if !c.RegistrationAck.GetAcceptance() {
				conn.Close()
				return nil, fmt.Errorf("sapient peer registration rejected: %v", c.RegistrationAck.GetAckResponseReason())
			}
		case *pb.SapientMessage_Error:
			conn.Close()
			return nil, fmt.Errorf("sapient peer registration error: %v", c.Error.GetErrorMessage())
		default:
			// No registration ack expected if middleware doesn't require peer registration
		}
	}

	return peer, nil
}

// SendTask sends a Task message to a specific edge node.
func (p *Peer) SendTask(destinationID string, t *pb.Task) error {
	msg := Msg(p.NodeID)
	msg.DestinationId = &destinationID
	msg.Content = &pb.SapientMessage_Task{Task: t}
	return p.Conn.Send(msg)
}

// SendAlertAck sends an AlertAck message.
func (p *Peer) SendAlertAck(destinationID string, aa *pb.AlertAck) error {
	msg := Msg(p.NodeID)
	msg.DestinationId = &destinationID
	msg.Content = &pb.SapientMessage_AlertAck{AlertAck: aa}
	return p.Conn.Send(msg)
}

// SendRegistrationAck sends a RegistrationAck to a specific node.
func (p *Peer) SendRegistrationAck(destinationID string, ack *pb.RegistrationAck) error {
	msg := Msg(p.NodeID)
	msg.DestinationId = &destinationID
	msg.Content = &pb.SapientMessage_RegistrationAck{RegistrationAck: ack}
	return p.Conn.Send(msg)
}

// Recv reads the next message from the middleware.
// Messages will be Registration, StatusReport, DetectionReport, Alert, or TaskAck
// from connected edge nodes.
func (p *Peer) Recv() (*pb.SapientMessage, error) {
	return p.Conn.Recv()
}

// Close closes the connection.
func (p *Peer) Close() error {
	return p.Conn.Close()
}

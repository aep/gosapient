package sapient

import (
	"fmt"

	pb "sapient/pkg/sapientpb"
)

// Child represents a SAPIENT edge node (sensor/effector) connection.
// It connects to the middleware as a Child and handles the registration
// handshake, status reporting, and detection reporting lifecycle.
type Child struct {
	Conn   *Conn
	NodeID string
}

// DialChild connects to a SAPIENT middleware Child port and performs registration.
// Returns after receiving a successful RegistrationAck.
func DialChild(addr string, reg *pb.Registration) (*Child, error) {
	conn, err := Dial(addr)
	if err != nil {
		return nil, err
	}

	nodeID := NewUUID()
	child := &Child{Conn: conn, NodeID: nodeID}

	msg := Msg(nodeID)
	msg.Content = &pb.SapientMessage_Registration{Registration: reg}

	if err := conn.Send(msg); err != nil {
		conn.Close()
		return nil, fmt.Errorf("sapient child send registration: %w", err)
	}

	reply, err := conn.Recv()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("sapient child recv ack: %w", err)
	}

	switch c := reply.GetContent().(type) {
	case *pb.SapientMessage_RegistrationAck:
		if !c.RegistrationAck.GetAcceptance() {
			conn.Close()
			return nil, fmt.Errorf("sapient child registration rejected: %v", c.RegistrationAck.GetAckResponseReason())
		}
	case *pb.SapientMessage_Error:
		conn.Close()
		return nil, fmt.Errorf("sapient child registration error: %v", c.Error.GetErrorMessage())
	default:
		conn.Close()
		return nil, fmt.Errorf("sapient child unexpected response: %s", ContentType(reply))
	}

	return child, nil
}

// SendStatus sends a StatusReport message.
func (c *Child) SendStatus(sr *pb.StatusReport) error {
	msg := Msg(c.NodeID)
	msg.Content = &pb.SapientMessage_StatusReport{StatusReport: sr}
	return c.Conn.Send(msg)
}

// SendDetection sends a DetectionReport message.
func (c *Child) SendDetection(dr *pb.DetectionReport) error {
	msg := Msg(c.NodeID)
	msg.Content = &pb.SapientMessage_DetectionReport{DetectionReport: dr}
	return c.Conn.Send(msg)
}

// SendAlert sends an Alert message.
func (c *Child) SendAlert(a *pb.Alert) error {
	msg := Msg(c.NodeID)
	msg.Content = &pb.SapientMessage_Alert{Alert: a}
	return c.Conn.Send(msg)
}

// SendTaskAck sends a TaskAck message.
func (c *Child) SendTaskAck(ta *pb.TaskAck) error {
	msg := Msg(c.NodeID)
	msg.Content = &pb.SapientMessage_TaskAck{TaskAck: ta}
	return c.Conn.Send(msg)
}

// Recv reads the next message from the middleware (typically a Task).
func (c *Child) Recv() (*pb.SapientMessage, error) {
	return c.Conn.Recv()
}

// Close closes the connection.
func (c *Child) Close() error {
	return c.Conn.Close()
}

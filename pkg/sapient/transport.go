// Package sapient implements the SAPIENT protocol (BSI Flex 335 v2.0).
//
// It provides a TCP transport layer with the length-prefixed framing specified in AEDP-4869 §6.3,
// and typed client connections for the Child (edge node/GCS), Peer (fusion node/C2),
// and Parent (higher echelon) roles.
package sapient

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	pb "sapient/pkg/sapientpb"

	"google.golang.org/protobuf/proto"
)

const (
	headerSize    = 4
	maxMessageLen = 1024 * 1024 // 1 MB default, matches Apex messageMaxSizeKb
)

// Conn wraps a TCP connection with SAPIENT length-prefixed framing.
// Messages are serialized as: [4-byte little-endian length][protobuf bytes].
type Conn struct {
	conn net.Conn
	mu   sync.Mutex // serializes writes
}

// NewConn wraps an existing net.Conn for SAPIENT message exchange.
func NewConn(conn net.Conn) *Conn {
	return &Conn{conn: conn}
}

// Dial connects to a SAPIENT node at addr (host:port) and returns a Conn.
func Dial(addr string) (*Conn, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("sapient dial %s: %w", addr, err)
	}
	return NewConn(conn), nil
}

// SendRaw sends pre-serialized protobuf bytes with the length-prefix header.
func (c *Conn) SendRaw(data []byte) error {
	header := make([]byte, headerSize)
	binary.LittleEndian.PutUint32(header, uint32(len(data)))

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := c.conn.Write(header); err != nil {
		return fmt.Errorf("sapient write header: %w", err)
	}
	if _, err := c.conn.Write(data); err != nil {
		return fmt.Errorf("sapient write body: %w", err)
	}
	return nil
}

// Send serializes and sends a v2 SapientMessage.
func (c *Conn) Send(msg *pb.SapientMessage) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("sapient marshal: %w", err)
	}
	return c.SendRaw(data)
}

// RecvRaw reads the next framed message and returns the raw protobuf bytes.
func (c *Conn) RecvRaw() ([]byte, error) {
	header := make([]byte, headerSize)
	if _, err := io.ReadFull(c.conn, header); err != nil {
		return nil, fmt.Errorf("sapient read header: %w", err)
	}

	length := binary.LittleEndian.Uint32(header)
	if length > maxMessageLen {
		return nil, fmt.Errorf("sapient message too large: %d bytes", length)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(c.conn, data); err != nil {
		return nil, fmt.Errorf("sapient read body: %w", err)
	}
	return data, nil
}

// Recv reads and deserializes the next message as a v2 SapientMessage.
func (c *Conn) Recv() (*pb.SapientMessage, error) {
	data, err := c.RecvRaw()
	if err != nil {
		return nil, err
	}
	msg := &pb.SapientMessage{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return nil, fmt.Errorf("sapient unmarshal: %w", err)
	}
	return msg, nil
}

// Close closes the underlying TCP connection.
func (c *Conn) Close() error {
	return c.conn.Close()
}

// RemoteAddr returns the remote address of the connection.
func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// LocalAddr returns the local address of the connection.
func (c *Conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// SendStatus sends a StatusReport from nodeID.
func (c *Conn) SendStatus(nodeID string, sr *pb.StatusReport) error {
	msg := Msg(nodeID)
	msg.Content = &pb.SapientMessage_StatusReport{StatusReport: sr}
	return c.Send(msg)
}

// SendDetection sends a DetectionReport from nodeID.
func (c *Conn) SendDetection(nodeID string, dr *pb.DetectionReport) error {
	msg := Msg(nodeID)
	msg.Content = &pb.SapientMessage_DetectionReport{DetectionReport: dr}
	return c.Send(msg)
}

// SendAlert sends an Alert from nodeID.
func (c *Conn) SendAlert(nodeID string, a *pb.Alert) error {
	msg := Msg(nodeID)
	msg.Content = &pb.SapientMessage_Alert{Alert: a}
	return c.Send(msg)
}

// SendTaskAck sends a TaskAck from nodeID.
func (c *Conn) SendTaskAck(nodeID string, ta *pb.TaskAck) error {
	msg := Msg(nodeID)
	msg.Content = &pb.SapientMessage_TaskAck{TaskAck: ta}
	return c.Send(msg)
}

// SendTask sends a Task from nodeID to destinationID.
func (c *Conn) SendTask(nodeID, destinationID string, t *pb.Task) error {
	msg := Msg(nodeID)
	msg.DestinationId = &destinationID
	msg.Content = &pb.SapientMessage_Task{Task: t}
	return c.Send(msg)
}

// SendAlertAck sends an AlertAck from nodeID to destinationID.
func (c *Conn) SendAlertAck(nodeID, destinationID string, aa *pb.AlertAck) error {
	msg := Msg(nodeID)
	msg.DestinationId = &destinationID
	msg.Content = &pb.SapientMessage_AlertAck{AlertAck: aa}
	return c.Send(msg)
}

// IsConnectionClosed returns true if the error indicates a closed connection.
func IsConnectionClosed(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) {
		return true
	}
	var opErr *net.OpError
	return errors.As(err, &opErr)
}

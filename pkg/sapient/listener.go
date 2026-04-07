package sapient

import (
	"fmt"
	"net"
)

// Listener accepts incoming SAPIENT TCP connections.
type Listener struct {
	ln net.Listener
}

// Listen starts a SAPIENT TCP listener on addr (e.g. ":5020").
func Listen(addr string) (*Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("sapient listen %s: %w", addr, err)
	}
	return &Listener{ln: ln}, nil
}

// Accept waits for and returns the next SAPIENT connection.
func (l *Listener) Accept() (*Conn, error) {
	conn, err := l.ln.Accept()
	if err != nil {
		return nil, fmt.Errorf("sapient accept: %w", err)
	}
	return NewConn(conn), nil
}

// Addr returns the listener's network address (useful when using ":0" for a random port).
func (l *Listener) Addr() net.Addr {
	return l.ln.Addr()
}

// Close stops the listener.
func (l *Listener) Close() error {
	return l.ln.Close()
}

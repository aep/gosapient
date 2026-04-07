package sapient_test

import (
	"net"
	"os/exec"
	"testing"

	"sapient/pkg/sapient"
)

// TestPythonToGoWireCompat starts a Go SAPIENT listener, then runs the Dstl
// Python reference implementation to send messages directly to it.
// No Apex middleware involved — this tests raw wire compatibility between
// the Go and Python protobuf serializations over the SAPIENT framing protocol.
func TestPythonToGoWireCompat(t *testing.T) {
	if _, err := exec.LookPath("uv"); err != nil {
		t.Skip("uv not found")
	}

	ln, err := sapient.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	host, port, _ := net.SplitHostPort(ln.Addr().String())
	t.Logf("Go listening on %s:%s", host, port)

	type result struct {
		msgs []string
		err  error
	}
	done := make(chan result, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			done <- result{err: err}
			return
		}
		defer conn.Close()

		var msgs []string
		for {
			msg, err := conn.Recv()
			if err != nil {
				if sapient.IsConnectionClosed(err) {
					break
				}
				done <- result{err: err}
				return
			}
			msgs = append(msgs, sapient.ContentType(msg))
		}
		done <- result{msgs: msgs}
	}()

	cmd := exec.Command("uv", "run", "../../tools/send-to-go.py", host, port)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Python failed: %v\n%s", err, out)
	}

	r := <-done
	if r.err != nil {
		t.Fatalf("Go receiver: %v", r.err)
	}

	expected := []string{"registration", "status_report", "detection_report", "alert"}
	if len(r.msgs) != len(expected) {
		t.Fatalf("expected %d messages, got %d: %v", len(expected), len(r.msgs), r.msgs)
	}
	for i, exp := range expected {
		if r.msgs[i] != exp {
			t.Errorf("message %d: expected %s, got %s", i, exp, r.msgs[i])
		} else {
			t.Logf("Python→Go: %s OK", exp)
		}
	}
}

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sync/atomic"

	"github.com/aep/gosapient/pkg/sapient"
	pb "github.com/aep/gosapient/pkg/sapientpb"

	"google.golang.org/protobuf/encoding/protojson"
)

func serverCmd(args []string) {
	fs := flag.NewFlagSet("server", flag.ExitOnError)
	addr := fs.String("addr", ":5020", "listen address")
	ack := fs.Bool("ack", true, "send RegistrationAck to registering nodes")
	json := fs.Bool("json", false, "print messages as JSON (default: summary line)")
	fs.Parse(args)

	ln, err := sapient.Listen(*addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		os.Exit(1)
	}

	myID := sapient.NewUUID()
	log.Printf("listening on %s (node_id=%s)", ln.Addr(), myID)

	var connID atomic.Int64

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			continue
		}
		id := connID.Add(1)
		log.Printf("[conn %d] connected from %s", id, conn.RemoteAddr())
		go handleConn(conn, id, myID, *ack, *json)
	}
}

func handleConn(conn *sapient.Conn, id int64, myID string, doAck, asJSON bool) {
	defer conn.Close()

	var nodeID string
	var isV1 bool

	for {
		msg, err := conn.Recv()
		if err != nil {
			if sapient.IsConnectionClosed(err) {
				log.Printf("[conn %d] disconnected", id)
			} else {
				log.Printf("[conn %d] recv error: %v", id, err)
			}
			return
		}

		ct := sapient.ContentType(msg)
		from := msg.GetNodeId()
		dest := msg.GetDestinationId()

		if ct == "registration" && nodeID == "" {
			nodeID = from
			icd := msg.GetRegistration().GetIcdVersion()
			isV1 = icd != "" && icd != "BSI Flex 335 v2.0"
			version := "v2"
			if isV1 {
				version = fmt.Sprintf("v1 (icd=%s)", icd)
			}
			log.Printf("[conn %d] registered as %s (%s) [%s]",
				id, msg.GetRegistration().GetName(), from, version)

			if doAck {
				var err error
				if isV1 {
					err = sapient.AckV1(conn, myID, from, true, "accepted")
				} else {
					err = sapient.Ack(conn, myID, from, true)
				}
				if err != nil {
					log.Printf("[conn %d] ack error: %v", id, err)
					return
				}
			}
		}

		if asJSON {
			out, _ := protojson.MarshalOptions{
				Multiline: true,
				Indent:    "  ",
			}.Marshal(msg)
			fmt.Printf("[conn %d] %s from %s:\n%s\n", id, ct, from, out)
		} else {
			line := fmt.Sprintf("[conn %d] %s from=%s", id, ct, from)
			if dest != "" {
				line += fmt.Sprintf(" dest=%s", dest)
			}
			if isV1 {
				line += " [v1]"
			}
			line += " " + summarize(msg)
			fmt.Println(line)
		}
	}
}

func summarize(msg *pb.SapientMessage) string {
	switch c := msg.GetContent().(type) {
	case *pb.SapientMessage_Registration:
		r := c.Registration
		types := []string{}
		for _, nd := range r.GetNodeDefinition() {
			types = append(types, nd.GetNodeType().String())
		}
		return fmt.Sprintf("name=%q types=%v icd=%q", r.GetName(), types, r.GetIcdVersion())

	case *pb.SapientMessage_RegistrationAck:
		return fmt.Sprintf("acceptance=%v", c.RegistrationAck.GetAcceptance())

	case *pb.SapientMessage_StatusReport:
		sr := c.StatusReport
		s := fmt.Sprintf("system=%s info=%s mode=%q",
			sr.GetSystem(), sr.GetInfo(), sr.GetMode())
		if loc := sr.GetNodeLocation(); loc != nil {
			s += fmt.Sprintf(" loc=(%.6f,%.6f,%.1f)", loc.GetX(), loc.GetY(), loc.GetZ())
		}
		return s

	case *pb.SapientMessage_DetectionReport:
		dr := c.DetectionReport
		s := fmt.Sprintf("report=%s object=%s conf=%.2f",
			dr.GetReportId(), dr.GetObjectId(), dr.GetDetectionConfidence())
		if loc := dr.GetLocation(); loc != nil {
			s += fmt.Sprintf(" loc=(%.6f,%.6f,%.1f)", loc.GetX(), loc.GetY(), loc.GetZ())
		}
		for _, cl := range dr.GetClassification() {
			s += fmt.Sprintf(" class=%q/%.2f", cl.GetType(), cl.GetConfidence())
		}
		return s

	case *pb.SapientMessage_Task:
		t := c.Task
		s := fmt.Sprintf("task=%s control=%s", t.GetTaskId(), t.GetControl())
		if cmd := t.GetCommand(); cmd != nil {
			switch {
			case cmd.GetRequest() != "":
				s += fmt.Sprintf(" request=%q", cmd.GetRequest())
			case cmd.GetModeChange() != "":
				s += fmt.Sprintf(" mode_change=%q", cmd.GetModeChange())
			case cmd.GetLookAt() != nil:
				s += " command=look_at"
			case cmd.GetFollow() != nil:
				s += fmt.Sprintf(" follow=%s", cmd.GetFollow().GetFollowObjectId())
			case cmd.GetPatrol() != nil:
				s += " command=patrol"
			case cmd.GetMoveTo() != nil:
				s += " command=move_to"
			}
		}
		return s

	case *pb.SapientMessage_TaskAck:
		ta := c.TaskAck
		return fmt.Sprintf("task=%s status=%s", ta.GetTaskId(), ta.GetTaskStatus())

	case *pb.SapientMessage_Alert:
		a := c.Alert
		return fmt.Sprintf("alert=%s type=%s priority=%s desc=%q",
			a.GetAlertId(), a.GetAlertType(), a.GetPriority(), a.GetDescription())

	case *pb.SapientMessage_AlertAck:
		aa := c.AlertAck
		return fmt.Sprintf("alert=%s status=%s", aa.GetAlertId(), aa.GetAlertAckStatus())

	case *pb.SapientMessage_Error:
		return fmt.Sprintf("errors=%v", c.Error.GetErrorMessage())

	default:
		return ""
	}
}

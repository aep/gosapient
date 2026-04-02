package sapient_test

import (
	"os"
	"testing"
	"time"

	"sapient/pkg/sapient"
	pb "sapient/pkg/sapientpb"
)

func apexAddr(envKey, fallback string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return fallback
}

var (
	childAddr = apexAddr("APEX_CHILD_ADDR", "localhost:5020")
	peerAddr  = apexAddr("APEX_PEER_ADDR", "localhost:5001")
)

func testRegistration() *pb.Registration {
	return &pb.Registration{
		IcdVersion: strp("BSI Flex 335 v2.0"),
		NodeDefinition: []*pb.Registration_NodeDefinition{{
			NodeType:    pb.Registration_NODE_TYPE_RADAR.Enum(),
			NodeSubType: []string{"Test FMCW Radar"},
		}},
		Name:      strp("Test Radar"),
		ShortName: strp("TEST1"),
		Capabilities: []*pb.Registration_Capability{{
			Category: strp("FMCW"),
			Type:     strp("Bandwidth"),
			Value:    strp("10"),
			Units:    strp("MHz"),
		}},
		StatusDefinition: &pb.Registration_StatusDefinition{
			StatusInterval: &pb.Registration_Duration{
				Units: pb.Registration_TIME_UNITS_SECONDS.Enum(),
				Value: fp(5.0),
			},
			LocationDefinition: locationType(),
			StatusReport: []*pb.Registration_StatusReport{{
				Category: pb.Registration_STATUS_REPORT_CATEGORY_SENSOR.Enum(),
				Type:     strp("operational"),
			}},
		},
		ModeDefinition: []*pb.Registration_ModeDefinition{{
			ModeName: strp("Surveillance"),
			ModeType: pb.Registration_MODE_TYPE_PERMANENT.Enum(),
			SettleTime: &pb.Registration_Duration{
				Units: pb.Registration_TIME_UNITS_SECONDS.Enum(),
				Value: fp(5.0),
			},
			DetectionDefinition: []*pb.Registration_DetectionDefinition{{
				LocationType: locationType(),
				DetectionClassDefinition: []*pb.Registration_DetectionClassDefinition{{
					ClassDefinition: []*pb.Registration_ClassDefinition{{
						Type: strp("Air Vehicle"),
					}},
				}},
			}},
			Task: &pb.Registration_TaskDefinition{
				ConcurrentTasks: ip(1),
				RegionDefinition: &pb.Registration_RegionDefinition{
					RegionType: []pb.Registration_RegionType{pb.Registration_REGION_TYPE_AREA_OF_INTEREST},
					SettleTime: &pb.Registration_Duration{
						Units: pb.Registration_TIME_UNITS_SECONDS.Enum(),
						Value: fp(5.0),
					},
					RegionArea: []*pb.Registration_LocationType{locationType()},
				},
				Command: []*pb.Registration_Command{{
					Type:  pb.Registration_COMMAND_TYPE_REQUEST.Enum(),
					Units: strp("Start, Stop"),
					CompletionTime: &pb.Registration_Duration{
						Units: pb.Registration_TIME_UNITS_SECONDS.Enum(),
						Value: fp(5.0),
					},
				}},
			},
		}},
		ConfigData: []*pb.Registration_ConfigurationData{{
			Manufacturer: "TestCorp",
			Model:        "TestRadar-1",
		}},
	}
}

func locationType() *pb.Registration_LocationType {
	return &pb.Registration_LocationType{
		CoordinatesOneof: &pb.Registration_LocationType_LocationUnits{
			LocationUnits: pb.LocationCoordinateSystem_LOCATION_COORDINATE_SYSTEM_LAT_LNG_DEG_M,
		},
		DatumOneof: &pb.Registration_LocationType_LocationDatum{
			LocationDatum: pb.LocationDatum_LOCATION_DATUM_WGS84_E,
		},
	}
}

// recvExpect reads messages until one matches the expected type AND comes from
// the expected node. This handles Apex replaying cached messages from previous
// test runs (stale registrations, status reports).
func recvExpect(t *testing.T, conn *sapient.Conn, expected string, fromNodeID ...string) *pb.SapientMessage {
	t.Helper()
	for {
		done := make(chan *pb.SapientMessage, 1)
		errc := make(chan error, 1)
		go func() {
			msg, err := conn.Recv()
			if err != nil {
				errc <- err
				return
			}
			done <- msg
		}()

		select {
		case msg := <-done:
			ct := sapient.ContentType(msg)
			if ct != expected {
				if len(fromNodeID) > 0 {
					t.Logf("draining %s from %s (waiting for %s)", ct, msg.GetNodeId(), expected)
					continue
				}
				t.Fatalf("expected %s, got %s", expected, ct)
			}
			if len(fromNodeID) > 0 && msg.GetNodeId() != fromNodeID[0] {
				t.Logf("draining %s from stale node %s", ct, msg.GetNodeId())
				continue
			}
			return msg
		case err := <-errc:
			t.Fatalf("recv error waiting for %s: %v", expected, err)
			return nil
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for %s", expected)
			return nil
		}
	}
}

func TestChildRegistration(t *testing.T) {
	child, err := sapient.DialChild(childAddr, testRegistration())
	if err != nil {
		t.Fatalf("DialChild: %v", err)
	}
	defer child.Close()
	t.Logf("Registered node_id=%s", child.NodeID)
}

func TestEndToEnd(t *testing.T) {
	// Connect Peer first so it receives forwarded messages
	peer, err := sapient.DialPeer(peerAddr, nil)
	if err != nil {
		t.Fatalf("DialPeer: %v", err)
	}
	defer peer.Close()

	// Connect and register Child
	child, err := sapient.DialChild(childAddr, testRegistration())
	if err != nil {
		t.Fatalf("DialChild: %v", err)
	}
	defer child.Close()

	// --- Registration flows Child→Apex→Peer ---
	// recvExpect with child.NodeID drains stale cached messages from prior tests
	msg := recvExpect(t, peer.Conn, "registration", child.NodeID)
	reg := msg.GetRegistration()
	t.Logf("Peer got registration: name=%q type=%s", reg.GetName(), reg.GetNodeDefinition()[0].GetNodeType())

	// --- StatusReport flows Child→Apex→Peer ---
	sr := sapient.NewStatusReport(
		pb.StatusReport_SYSTEM_OK,
		pb.StatusReport_INFO_NEW,
		"Surveillance",
	).
		Location(51.5, -1.2, 30.0).
		Power(pb.StatusReport_POWERSOURCE_MAINS, pb.StatusReport_POWERSTATUS_OK, 100).
		FieldOfView(180, 10, 120, 20).
		Build()

	if err := child.SendStatus(sr); err != nil {
		t.Fatalf("SendStatus: %v", err)
	}

	msg = recvExpect(t, peer.Conn, "status_report", child.NodeID)
	gotSR := msg.GetStatusReport()
	t.Logf("Peer got status: system=%s mode=%s loc=(%v,%v)",
		gotSR.GetSystem(), gotSR.GetMode(),
		gotSR.GetNodeLocation().GetX(), gotSR.GetNodeLocation().GetY())

	// --- DetectionReport flows Child→Apex→Peer ---
	objectID := sapient.NewULID()
	det := sapient.NewDetection(objectID).
		Location(51.501, -1.199, 50.0).
		Confidence(0.85).
		SubClassification("Air Vehicle", 0.9, "UAV Rotary Wing", 0.85, 1).
		Behaviour("Active", 0.95).
		Velocity(5.0, 3.0, 0.5).
		TrackInfo("speed", "6.2").
		Build()

	if err := child.SendDetection(det); err != nil {
		t.Fatalf("SendDetection: %v", err)
	}

	msg = recvExpect(t, peer.Conn, "detection_report", child.NodeID)
	gotDet := msg.GetDetectionReport()
	t.Logf("Peer got detection: object=%s conf=%.2f class=%s",
		gotDet.GetObjectId(), gotDet.GetDetectionConfidence(),
		gotDet.GetClassification()[0].GetType())

	// --- Task flows Peer→Apex→Child ---
	task := sapient.NewTask(pb.Task_CONTROL_START).
		Name("Look at detection").
		Request("Start").
		Region("AOI-1", pb.Task_REGION_TYPE_AREA_OF_INTEREST, 51.501, -1.199, 0).
		Build()

	if err := peer.SendTask(child.NodeID, task); err != nil {
		t.Fatalf("SendTask: %v", err)
	}

	msg = recvExpect(t, child.Conn, "task")
	gotTask := msg.GetTask()
	t.Logf("Child got task: id=%s control=%s command=%s",
		gotTask.GetTaskId(), gotTask.GetControl(),
		gotTask.GetCommand().GetRequest())

	// --- TaskAck flows Child→Apex→Peer ---
	ack := sapient.NewTaskAck(gotTask.GetTaskId(), pb.TaskAck_TASK_STATUS_ACCEPTED, "Task accepted")
	if err := child.SendTaskAck(ack); err != nil {
		t.Fatalf("SendTaskAck: %v", err)
	}

	msg = recvExpect(t, peer.Conn, "task_ack", child.NodeID)
	gotAck := msg.GetTaskAck()
	t.Logf("Peer got task_ack: id=%s status=%s", gotAck.GetTaskId(), gotAck.GetTaskStatus())

	if gotAck.GetTaskId() != gotTask.GetTaskId() {
		t.Errorf("task_id mismatch: sent %s, got %s", gotTask.GetTaskId(), gotAck.GetTaskId())
	}

	// --- Alert flows Child→Apex→Peer ---
	alert := sapient.NewAlert(
		pb.Alert_ALERT_TYPE_WARNING,
		pb.Alert_ALERT_STATUS_ACTIVE,
		pb.Alert_DISCRETE_PRIORITY_HIGH,
		"Hostile UAV confirmed",
	)
	if err := child.SendAlert(alert); err != nil {
		t.Fatalf("SendAlert: %v", err)
	}

	msg = recvExpect(t, peer.Conn, "alert", child.NodeID)
	gotAlert := msg.GetAlert()
	t.Logf("Peer got alert: id=%s type=%s priority=%s desc=%q",
		gotAlert.GetAlertId(), gotAlert.GetAlertType(),
		gotAlert.GetPriority(), gotAlert.GetDescription())

	// --- TaskAck COMPLETED flows Child→Apex→Peer ---
	completed := sapient.NewTaskAck(gotTask.GetTaskId(), pb.TaskAck_TASK_STATUS_COMPLETED, "Done")
	if err := child.SendTaskAck(completed); err != nil {
		t.Fatalf("SendTaskAck completed: %v", err)
	}

	msg = recvExpect(t, peer.Conn, "task_ack", child.NodeID)
	t.Logf("Peer got task_ack completed: status=%s", msg.GetTaskAck().GetTaskStatus())

	// --- StatusReport GOODBYE ---
	goodbye := sapient.NewStatusReport(
		pb.StatusReport_SYSTEM_GOODBYE,
		pb.StatusReport_INFO_NEW,
		"Surveillance",
	).Build()

	if err := child.SendStatus(goodbye); err != nil {
		t.Fatalf("SendStatus goodbye: %v", err)
	}

	msg = recvExpect(t, peer.Conn, "status_report", child.NodeID)
	t.Logf("Peer got goodbye: system=%s", msg.GetStatusReport().GetSystem())
}

func strp(s string) *string { return &s }
func fp(f float32) *float32 { return &f }
func ip(i int32) *int32     { return &i }

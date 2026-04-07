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

// dialChild connects as Child, registers, returns conn + nodeID.
func dialChild(t *testing.T) (*sapient.Conn, string) {
	t.Helper()
	conn, err := sapient.Dial(childAddr)
	if err != nil {
		t.Fatalf("dial child: %v", err)
	}
	nodeID, err := sapient.Register(conn, testRegistration())
	if err != nil {
		conn.Close()
		t.Fatalf("register: %v", err)
	}
	return conn, nodeID
}

// recvExpect reads messages until one matches the expected type from the expected node.
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
			if len(fromNodeID) > 0 && msg.GetNodeId() != fromNodeID[0] {
				continue
			}
			if ct != expected {
				if len(fromNodeID) > 0 {
					continue
				}
				t.Fatalf("expected %s, got %s", expected, ct)
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

func TestRegistration(t *testing.T) {
	conn, nodeID := dialChild(t)
	defer conn.Close()
	t.Logf("Registered node_id=%s", nodeID)
}

func TestEndToEnd(t *testing.T) {
	peer, err := sapient.Dial(peerAddr)
	if err != nil {
		t.Fatalf("dial peer: %v", err)
	}
	defer peer.Close()

	child, childID := dialChild(t)
	defer child.Close()

	// Registration
	msg := recvExpect(t, peer, "registration", childID)
	t.Logf("Peer got registration: name=%q", msg.GetRegistration().GetName())

	// StatusReport
	child.SendStatus(childID, sapient.NewStatusReport(pb.StatusReport_SYSTEM_OK, pb.StatusReport_INFO_NEW, "Surveillance").
		Location(51.5, -1.2, 30.0).
		Power(pb.StatusReport_POWERSOURCE_MAINS, pb.StatusReport_POWERSTATUS_OK, 100).
		Build())
	msg = recvExpect(t, peer, "status_report", childID)
	t.Logf("Peer got status: system=%s", msg.GetStatusReport().GetSystem())

	// DetectionReport
	child.SendDetection(childID, sapient.NewDetection(sapient.NewULID()).
		Location(51.501, -1.199, 50.0).
		Confidence(0.85).
		SubClassification("Air Vehicle", 0.9, "UAV Rotary Wing", 0.85, 1).
		Behaviour("Active", 0.95).
		Velocity(5.0, 3.0, 0.5).
		Build())
	msg = recvExpect(t, peer, "detection_report", childID)
	t.Logf("Peer got detection: class=%s", msg.GetDetectionReport().GetClassification()[0].GetType())

	// Task (Peer→Child)
	peerID := sapient.NewUUID()
	task := sapient.NewTask(pb.Task_CONTROL_START).Request("Start").
		Region("AOI-1", pb.Task_REGION_TYPE_AREA_OF_INTEREST, 51.501, -1.199, 0).
		Build()
	peer.SendTask(peerID, childID, task)
	msg = recvExpect(t, child, "task")
	t.Logf("Child got task: control=%s", msg.GetTask().GetControl())

	// TaskAck
	child.SendTaskAck(childID, sapient.NewTaskAck(msg.GetTask().GetTaskId(), pb.TaskAck_TASK_STATUS_ACCEPTED, "OK"))
	msg = recvExpect(t, peer, "task_ack", childID)
	t.Logf("Peer got task_ack: status=%s", msg.GetTaskAck().GetTaskStatus())

	// Alert
	child.SendAlert(childID, sapient.NewAlert(pb.Alert_ALERT_TYPE_WARNING, pb.Alert_ALERT_STATUS_ACTIVE, pb.Alert_DISCRETE_PRIORITY_HIGH, "Hostile UAV"))
	msg = recvExpect(t, peer, "alert", childID)
	t.Logf("Peer got alert: desc=%q", msg.GetAlert().GetDescription())

	// Goodbye
	child.SendStatus(childID, sapient.NewStatusReport(pb.StatusReport_SYSTEM_GOODBYE, pb.StatusReport_INFO_NEW, "Surveillance").Build())
	msg = recvExpect(t, peer, "status_report", childID)
	t.Logf("Peer got goodbye: system=%s", msg.GetStatusReport().GetSystem())
}

func strp(s string) *string { return &s }
func fp(f float32) *float32 { return &f }
func ip(i int32) *int32     { return &i }

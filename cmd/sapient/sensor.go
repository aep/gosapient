package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"sapient/pkg/sapient"
	pb "sapient/pkg/sapientpb"
)

func sensorCmd(args []string) {
	fs := flag.NewFlagSet("sensor", flag.ExitOnError)
	addr := fs.String("addr", "localhost:5020", "middleware or fusion node child address")
	name := fs.String("name", "Test Radar", "sensor name")
	nodeType := fs.String("type", "RADAR", "node type (RADAR, CAMERA, PASSIVE_RF, JAMMER, ...)")
	lat := fs.Float64("lat", 51.5, "sensor latitude")
	lng := fs.Float64("lng", -1.2, "sensor longitude")
	alt := fs.Float64("alt", 30.0, "sensor altitude")
	statusInterval := fs.Float64("status-interval", 5.0, "status report interval in seconds")
	detectInterval := fs.Float64("detect-interval", 1.0, "detection report interval in seconds")
	fs.Parse(args)

	conn, err := sapient.Dial(*addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	nt := parseNodeType(*nodeType)

	reg := &pb.Registration{
		IcdVersion: strPtr("BSI Flex 335 v2.0"),
		NodeDefinition: []*pb.Registration_NodeDefinition{{
			NodeType:    nt.Enum(),
			NodeSubType: []string{*name},
		}},
		Name:      strPtr(*name),
		ShortName: strPtr(*nodeType),
		Capabilities: []*pb.Registration_Capability{{
			Category: strPtr("sensor"),
			Type:     strPtr("range"),
			Value:    strPtr("5000"),
			Units:    strPtr("m"),
		}},
		StatusDefinition: &pb.Registration_StatusDefinition{
			StatusInterval: &pb.Registration_Duration{
				Units: pb.Registration_TIME_UNITS_SECONDS.Enum(),
				Value: f32Ptr(float32(*statusInterval)),
			},
			LocationDefinition: latLngType(),
			StatusReport: []*pb.Registration_StatusReport{{
				Category: pb.Registration_STATUS_REPORT_CATEGORY_SENSOR.Enum(),
				Type:     strPtr("operational"),
			}},
		},
		ModeDefinition: []*pb.Registration_ModeDefinition{{
			ModeName: strPtr("Default"),
			ModeType: pb.Registration_MODE_TYPE_PERMANENT.Enum(),
			SettleTime: &pb.Registration_Duration{
				Units: pb.Registration_TIME_UNITS_SECONDS.Enum(),
				Value: f32Ptr(1.0),
			},
			DetectionDefinition: []*pb.Registration_DetectionDefinition{{
				LocationType: latLngType(),
				DetectionClassDefinition: []*pb.Registration_DetectionClassDefinition{{
					ClassDefinition: []*pb.Registration_ClassDefinition{{Type: strPtr("Air Vehicle")}},
				}},
			}},
			Task: &pb.Registration_TaskDefinition{
				ConcurrentTasks: i32Ptr(1),
				RegionDefinition: &pb.Registration_RegionDefinition{
					RegionType: []pb.Registration_RegionType{pb.Registration_REGION_TYPE_AREA_OF_INTEREST},
					SettleTime: &pb.Registration_Duration{
						Units: pb.Registration_TIME_UNITS_SECONDS.Enum(),
						Value: f32Ptr(1.0),
					},
					RegionArea: []*pb.Registration_LocationType{latLngType()},
				},
				Command: []*pb.Registration_Command{{
					Type:  pb.Registration_COMMAND_TYPE_REQUEST.Enum(),
					Units: strPtr("Start, Stop"),
					CompletionTime: &pb.Registration_Duration{
						Units: pb.Registration_TIME_UNITS_SECONDS.Enum(),
						Value: f32Ptr(5.0),
					},
				}},
			},
		}},
		ConfigData: []*pb.Registration_ConfigurationData{{
			Manufacturer: "gosapient",
			Model:        "test-sensor",
		}},
	}

	nodeID, err := sapient.Register(conn, reg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "register: %v\n", err)
		os.Exit(1)
	}
	log.Printf("registered as %s (node_id=%s)", *name, nodeID)

	// Listen for tasks in background
	go func() {
		for {
			msg, err := conn.Recv()
			if err != nil {
				if sapient.IsConnectionClosed(err) {
					return
				}
				log.Printf("recv: %v", err)
				return
			}
			ct := sapient.ContentType(msg)
			log.Printf("received %s: %s", ct, summarize(msg))

			if ct == "task" {
				conn.SendTaskAck(nodeID, sapient.NewTaskAck(
					msg.GetTask().GetTaskId(),
					pb.TaskAck_TASK_STATUS_ACCEPTED,
					"accepted by test sensor",
				))
			}
		}
	}()

	// Send status reports
	statusTicker := time.NewTicker(time.Duration(*statusInterval * float64(time.Second)))
	defer statusTicker.Stop()

	// Send detections
	detectTicker := time.NewTicker(time.Duration(*detectInterval * float64(time.Second)))
	defer detectTicker.Stop()

	objectID := sapient.NewULID()
	tick := 0

	for {
		select {
		case <-statusTicker.C:
			sr := sapient.NewStatusReport(pb.StatusReport_SYSTEM_OK, pb.StatusReport_INFO_NEW, "Default").
				Location(*lat, *lng, *alt).
				Power(pb.StatusReport_POWERSOURCE_MAINS, pb.StatusReport_POWERSTATUS_OK, 100).
				Build()
			if err := conn.SendStatus(nodeID, sr); err != nil {
				log.Printf("send status: %v", err)
				return
			}
			log.Printf("sent status_report")

		case <-detectTicker.C:
			tick++
			// Simulate a drone circling at ~100m radius
			angle := float64(tick) * 0.1
			dLat := *lat + 0.001*math.Cos(angle)
			dLng := *lng + 0.001*math.Sin(angle)
			dAlt := 50.0 + 10.0*math.Sin(float64(tick)*0.05)

			det := sapient.NewDetection(objectID).
				Location(dLat, dLng, dAlt).
				Confidence(0.85).
				SubClassification("Air Vehicle", 0.9, "UAV Rotary Wing", 0.85, 1).
				Behaviour("Active", 0.95).
				Velocity(5.0*math.Sin(angle), 5.0*math.Cos(angle), 0.2*math.Cos(float64(tick)*0.05)).
				Build()
			if err := conn.SendDetection(nodeID, det); err != nil {
				log.Printf("send detection: %v", err)
				return
			}
			log.Printf("sent detection_report object=%s loc=(%.6f,%.6f,%.1f)", objectID, dLat, dLng, dAlt)
		}
	}
}

func parseNodeType(s string) pb.Registration_NodeType {
	full := "NODE_TYPE_" + s
	if v, ok := pb.Registration_NodeType_value[full]; ok {
		return pb.Registration_NodeType(v)
	}
	log.Printf("warning: unknown node type %q, using OTHER", s)
	return pb.Registration_NODE_TYPE_OTHER
}

func latLngType() *pb.Registration_LocationType {
	return &pb.Registration_LocationType{
		CoordinatesOneof: &pb.Registration_LocationType_LocationUnits{
			LocationUnits: pb.LocationCoordinateSystem_LOCATION_COORDINATE_SYSTEM_LAT_LNG_DEG_M,
		},
		DatumOneof: &pb.Registration_LocationType_LocationDatum{
			LocationDatum: pb.LocationDatum_LOCATION_DATUM_WGS84_E,
		},
	}
}

func strPtr(s string) *string    { return &s }
func f32Ptr(f float32) *float32  { return &f }
func i32Ptr(i int32) *int32      { return &i }

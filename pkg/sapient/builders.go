package sapient

import (
	pb "sapient/pkg/sapientpb"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// StatusReportBuilder helps construct StatusReport messages.
type StatusReportBuilder struct {
	sr *pb.StatusReport
}

// NewStatusReport starts building a StatusReport with required fields.
func NewStatusReport(system pb.StatusReport_System, info pb.StatusReport_Info, mode string) *StatusReportBuilder {
	id := NewULID()
	return &StatusReportBuilder{sr: &pb.StatusReport{
		ReportId: &id,
		System:   system.Enum(),
		Info:     info.Enum(),
		Mode:     &mode,
	}}
}

func (b *StatusReportBuilder) ActiveTask(taskID string) *StatusReportBuilder {
	b.sr.ActiveTaskId = &taskID
	return b
}

func (b *StatusReportBuilder) Power(source pb.StatusReport_PowerSource, status pb.StatusReport_PowerStatus, level int32) *StatusReportBuilder {
	b.sr.Power = &pb.StatusReport_Power{
		Source: source,
		Status: status,
		Level:  &level,
	}
	return b
}

func (b *StatusReportBuilder) Location(lat, lng, alt float64) *StatusReportBuilder {
	b.sr.NodeLocation = &pb.Location{
		X:                &lat,
		Y:                &lng,
		Z:                &alt,
		CoordinateSystem: pb.LocationCoordinateSystem_LOCATION_COORDINATE_SYSTEM_LAT_LNG_DEG_M.Enum(),
		Datum:            pb.LocationDatum_LOCATION_DATUM_WGS84_E.Enum(),
	}
	return b
}

func (b *StatusReportBuilder) FieldOfView(az, el, hExtent, vExtent float64) *StatusReportBuilder {
	b.sr.FieldOfView = &pb.LocationOrRangeBearing{
		FovOneof: &pb.LocationOrRangeBearing_RangeBearing{
			RangeBearing: &pb.RangeBearingCone{
				Azimuth:          &az,
				Elevation:        &el,
				HorizontalExtent: &hExtent,
				VerticalExtent:   &vExtent,
				CoordinateSystem: pb.RangeBearingCoordinateSystem_RANGE_BEARING_COORDINATE_SYSTEM_DEGREES_M.Enum(),
				Datum:            pb.RangeBearingDatum_RANGE_BEARING_DATUM_TRUE.Enum(),
			},
		},
	}
	return b
}

func (b *StatusReportBuilder) Status(level pb.StatusReport_StatusLevel, typ pb.StatusReport_StatusType, value string) *StatusReportBuilder {
	b.sr.Status = append(b.sr.Status, &pb.StatusReport_Status{
		StatusLevel: level.Enum(),
		StatusType:  typ.Enum(),
		StatusValue: &value,
	})
	return b
}

func (b *StatusReportBuilder) Build() *pb.StatusReport {
	return b.sr
}

// DetectionBuilder helps construct DetectionReport messages.
type DetectionBuilder struct {
	dr *pb.DetectionReport
}

// NewDetection starts building a DetectionReport. objectID should be consistent
// for the same tracked object across updates.
func NewDetection(objectID string) *DetectionBuilder {
	id := NewULID()
	return &DetectionBuilder{dr: &pb.DetectionReport{
		ReportId: &id,
		ObjectId: &objectID,
	}}
}

func (b *DetectionBuilder) TaskID(taskID string) *DetectionBuilder {
	b.dr.TaskId = &taskID
	return b
}

func (b *DetectionBuilder) Location(lat, lng, alt float64) *DetectionBuilder {
	b.dr.LocationOneof = &pb.DetectionReport_Location{
		Location: &pb.Location{
			X:                &lat,
			Y:                &lng,
			Z:                &alt,
			CoordinateSystem: pb.LocationCoordinateSystem_LOCATION_COORDINATE_SYSTEM_LAT_LNG_DEG_M.Enum(),
			Datum:            pb.LocationDatum_LOCATION_DATUM_WGS84_E.Enum(),
		},
	}
	return b
}

func (b *DetectionBuilder) RangeBearing(az, el, rng float64) *DetectionBuilder {
	b.dr.LocationOneof = &pb.DetectionReport_RangeBearing{
		RangeBearing: &pb.RangeBearing{
			Azimuth:          &az,
			Elevation:        &el,
			Range:            &rng,
			CoordinateSystem: pb.RangeBearingCoordinateSystem_RANGE_BEARING_COORDINATE_SYSTEM_DEGREES_M.Enum(),
			Datum:            pb.RangeBearingDatum_RANGE_BEARING_DATUM_TRUE.Enum(),
		},
	}
	return b
}

func (b *DetectionBuilder) Confidence(c float32) *DetectionBuilder {
	b.dr.DetectionConfidence = &c
	return b
}

func (b *DetectionBuilder) Classification(typ string, confidence float32) *DetectionBuilder {
	b.dr.Classification = append(b.dr.Classification, &pb.DetectionReport_DetectionReportClassification{
		Type:       &typ,
		Confidence: &confidence,
	})
	return b
}

func (b *DetectionBuilder) SubClassification(typ string, confidence float32, subType string, subConf float32, level int32) *DetectionBuilder {
	b.dr.Classification = append(b.dr.Classification, &pb.DetectionReport_DetectionReportClassification{
		Type:       &typ,
		Confidence: &confidence,
		SubClass: []*pb.DetectionReport_SubClass{{
			Type:       &subType,
			Confidence: &subConf,
			Level:      &level,
		}},
	})
	return b
}

func (b *DetectionBuilder) Behaviour(typ string, confidence float32) *DetectionBuilder {
	b.dr.Behaviour = append(b.dr.Behaviour, &pb.DetectionReport_Behaviour{
		Type:       &typ,
		Confidence: &confidence,
	})
	return b
}

func (b *DetectionBuilder) Velocity(east, north, up float64) *DetectionBuilder {
	b.dr.VelocityOneof = &pb.DetectionReport_EnuVelocity{
		EnuVelocity: &pb.ENUVelocity{
			EastRate:  &east,
			NorthRate: &north,
			UpRate:    &up,
		},
	}
	return b
}

func (b *DetectionBuilder) TrackInfo(typ, value string) *DetectionBuilder {
	b.dr.TrackInfo = append(b.dr.TrackInfo, &pb.DetectionReport_TrackObjectInfo{
		Type:  &typ,
		Value: &value,
	})
	return b
}

func (b *DetectionBuilder) ObjectInfo(typ, value string) *DetectionBuilder {
	b.dr.ObjectInfo = append(b.dr.ObjectInfo, &pb.DetectionReport_TrackObjectInfo{
		Type:  &typ,
		Value: &value,
	})
	return b
}

func (b *DetectionBuilder) Signal(amplitude, centreFreq float32) *DetectionBuilder {
	b.dr.Signal = append(b.dr.Signal, &pb.DetectionReport_Signal{
		Amplitude:       &amplitude,
		CentreFrequency: &centreFreq,
	})
	return b
}

func (b *DetectionBuilder) Build() *pb.DetectionReport {
	return b.dr
}

// TaskBuilder helps construct Task messages.
type TaskBuilder struct {
	t *pb.Task
}

// NewTask starts building a Task with a generated task_id.
func NewTask(control pb.Task_Control) *TaskBuilder {
	id := NewULID()
	return &TaskBuilder{t: &pb.Task{
		TaskId:  &id,
		Control: control.Enum(),
	}}
}

func (b *TaskBuilder) Name(name string) *TaskBuilder {
	b.t.TaskName = &name
	return b
}

func (b *TaskBuilder) TimeRange(start, end *timestamppb.Timestamp) *TaskBuilder {
	b.t.TaskStartTime = start
	b.t.TaskEndTime = end
	return b
}

func (b *TaskBuilder) Request(req string) *TaskBuilder {
	b.t.Command = &pb.Task_Command{
		Command: &pb.Task_Command_Request{Request: req},
	}
	return b
}

func (b *TaskBuilder) ModeChange(mode string) *TaskBuilder {
	b.t.Command = &pb.Task_Command{
		Command: &pb.Task_Command_ModeChange{ModeChange: mode},
	}
	return b
}

func (b *TaskBuilder) LookAt(lat, lng, alt float64) *TaskBuilder {
	b.t.Command = &pb.Task_Command{
		Command: &pb.Task_Command_LookAt{
			LookAt: &pb.LocationOrRangeBearing{
				FovOneof: &pb.LocationOrRangeBearing_LocationList{
					LocationList: &pb.LocationList{
						Locations: []*pb.Location{{
							X:                &lat,
							Y:                &lng,
							Z:                &alt,
							CoordinateSystem: pb.LocationCoordinateSystem_LOCATION_COORDINATE_SYSTEM_LAT_LNG_DEG_M.Enum(),
							Datum:            pb.LocationDatum_LOCATION_DATUM_WGS84_E.Enum(),
						}},
					},
				},
			},
		},
	}
	return b
}

func (b *TaskBuilder) Follow(objectID string) *TaskBuilder {
	b.t.Command = &pb.Task_Command{
		Command: &pb.Task_Command_Follow{
			Follow: &pb.FollowObject{
				FollowObjectId: objectID,
			},
		},
	}
	return b
}

func (b *TaskBuilder) Region(name string, typ pb.Task_RegionType, lat, lng, alt float64) *TaskBuilder {
	regionID := NewULID()
	b.t.Region = append(b.t.Region, &pb.Task_Region{
		Type:       typ.Enum(),
		RegionId:   &regionID,
		RegionName: &name,
		RegionArea: &pb.LocationOrRangeBearing{
			FovOneof: &pb.LocationOrRangeBearing_LocationList{
				LocationList: &pb.LocationList{
					Locations: []*pb.Location{{
						X:                &lat,
						Y:                &lng,
						Z:                &alt,
						CoordinateSystem: pb.LocationCoordinateSystem_LOCATION_COORDINATE_SYSTEM_LAT_LNG_DEG_M.Enum(),
						Datum:            pb.LocationDatum_LOCATION_DATUM_WGS84_E.Enum(),
					}},
				},
			},
		},
	})
	return b
}

func (b *TaskBuilder) Build() *pb.Task {
	return b.t
}

// NewTaskAck creates a TaskAck for the given task_id.
func NewTaskAck(taskID string, status pb.TaskAck_TaskStatus, reasons ...string) *pb.TaskAck {
	return &pb.TaskAck{
		TaskId:     &taskID,
		TaskStatus: status.Enum(),
		Reason:     reasons,
	}
}

// NewAlert creates an Alert with a generated alert_id.
func NewAlert(alertType pb.Alert_AlertType, status pb.Alert_AlertStatus, priority pb.Alert_DiscretePriority, description string) *pb.Alert {
	id := NewULID()
	return &pb.Alert{
		AlertId:     &id,
		AlertType:   alertType.Enum(),
		Status:      status.Enum(),
		Priority:    priority.Enum(),
		Description: &description,
	}
}

// NewAlertAck creates an AlertAck for the given alert_id.
func NewAlertAck(alertID string, status pb.AlertAck_AlertAckStatus, reasons ...string) *pb.AlertAck {
	return &pb.AlertAck{
		AlertId:        &alertID,
		AlertAckStatus: status.Enum(),
		Reason:         reasons,
	}
}

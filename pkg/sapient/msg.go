package sapient

import (
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	pb "github.com/aep/gosapient/pkg/sapientpb"

	"google.golang.org/protobuf/types/known/timestamppb"
)

const ICDVersion = "BSI Flex 335 v2.0"

var ulidEntropy = rand.New(rand.NewSource(time.Now().UnixNano()))

// NewUUID generates a new random UUID v4 string.
func NewUUID() string {
	return uuid.New().String()
}

// NewULID generates a new ULID string.
func NewULID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), ulidEntropy).String()
}

// Now returns the current time as a protobuf Timestamp.
func Now() *timestamppb.Timestamp {
	return timestamppb.Now()
}

// Msg creates a new SapientMessage with timestamp and node_id pre-filled.
func Msg(nodeID string) *pb.SapientMessage {
	return &pb.SapientMessage{
		Timestamp: Now(),
		NodeId:    &nodeID,
	}
}

// ContentType returns the name of the content oneof field set on a message,
// or "none" if no content is set.
func ContentType(msg *pb.SapientMessage) string {
	switch msg.GetContent().(type) {
	case *pb.SapientMessage_Registration:
		return "registration"
	case *pb.SapientMessage_RegistrationAck:
		return "registration_ack"
	case *pb.SapientMessage_StatusReport:
		return "status_report"
	case *pb.SapientMessage_DetectionReport:
		return "detection_report"
	case *pb.SapientMessage_Task:
		return "task"
	case *pb.SapientMessage_TaskAck:
		return "task_ack"
	case *pb.SapientMessage_Alert:
		return "alert"
	case *pb.SapientMessage_AlertAck:
		return "alert_ack"
	case *pb.SapientMessage_Error:
		return "error"
	default:
		return "none"
	}
}

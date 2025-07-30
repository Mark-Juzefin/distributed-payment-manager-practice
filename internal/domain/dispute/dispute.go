package dispute

import "time"

type Dispute struct {
	ID       string        `bson:"_id"`
	OrderID  string        `bson:"order_id"`
	Reason   string        `bson:"reason"`
	Status   DisputeStatus `bson:"status"`
	OpenedAt time.Time     `bson:"opened_at"`
	ClosedAt *time.Time    `bson:"closed_at"`
}

type DisputeStatus string

const (
	DisputeOpen   DisputeStatus = "open"
	DisputeClosed DisputeStatus = "closed"
)

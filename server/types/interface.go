package types

import (
	"time"
)

type UserID int64
type ActivityID int64
type ActivityType int64

const (
	ActivityUnknown ActivityType = iota
	ActivityOverallDay
)

type Activity struct {
	ID         ActivityID
	Type       ActivityType
	UTCDate    time.Time
	ActualTime time.Time
	Value      string
	RawValue   string
}

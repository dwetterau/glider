package types

import (
	"time"
)

type UserID int64
type ActivityID int64
type ActivityType int64

const (
	// WARNING: Only append to this list! The enum values are in a DB.
	ActivityUnknown ActivityType = iota
	ActivityOverallDay
	ActivityProgramming
	ActivityLaundry
	ActivityRunning
	ActivityMeetings
	ActivityReading
	ActivityYoga
	ActivityClimbing
	// Note: When adding to this, please also add to the `activities` command.
)

type Activity struct {
	ID          ActivityID
	Type        ActivityType
	UTCDate     time.Time
	ActualTime  time.Time
	Value       string
	Count       int64
	Duration    time.Duration
	RawMessages string
}

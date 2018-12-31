package db

import (
	"math/rand"
	"time"

	"github.com/dwetterau/glider/server/types"
)

type testImpl struct {
	users    map[string]types.UserID
	idToTZ   map[types.UserID]*time.Location
	activity map[types.UserID][]types.Activity
}

var _ Database = &testImpl{}

func TestOnlyMockImpl() Database {
	return &testImpl{
		users:    make(map[string]types.UserID),
		idToTZ:   make(map[types.UserID]*time.Location),
		activity: make(map[types.UserID][]types.Activity),
	}
}
func (t *testImpl) AddUser(fbID string, tz *time.Location) (types.UserID, *time.Location, error) {
	if val, ok := t.users[fbID]; ok {
		tz := t.idToTZ[val]
		return val, tz, nil
	}
	userID := types.UserID(rand.Int63())
	t.users[fbID] = userID
	t.idToTZ[userID] = tz
	return userID, tz, nil
}

func (t *testImpl) SetTimezone(userID types.UserID, tz *time.Location) error {
	t.idToTZ[userID] = tz
	return nil
}

func (t *testImpl) AddActivity(userID types.UserID, activity types.Activity) (types.ActivityID, error) {
	if activity.ID == 0 {
		activity.ID = types.ActivityID(rand.Int63())
	}
	t.activity[userID] = append(t.activity[userID], activity)
	return activity.ID, nil
}

func (t *testImpl) ActivityForUser(userID types.UserID) ([]types.Activity, error) {
	return t.activity[userID], nil
}

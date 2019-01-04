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
func (t *testImpl) AddOrGetUser(fbID string, tz *time.Location) (types.UserID, *time.Location, bool, error) {
	if val, ok := t.users[fbID]; ok {
		tz := t.idToTZ[val]
		return val, tz, false, nil
	}
	userID := types.UserID(rand.Int63())
	t.users[fbID] = userID
	t.idToTZ[userID] = tz
	return userID, tz, true, nil
}

func (t *testImpl) SetTimezone(userID types.UserID, tz *time.Location) error {
	t.idToTZ[userID] = tz
	return nil
}

func (t *testImpl) AddOrUpdateActivity(userID types.UserID, activity types.Activity) (types.ActivityID, error) {
	if activity.ID == 0 {
		activity.ID = types.ActivityID(rand.Int63())
	} else {
		// Find the existing activity
		index := 0
		for i, a := range t.activity[userID] {
			if a.ID == activity.ID {
				index = i
				break
			}
		}
		t.activity[userID][index] = activity
	}
	t.activity[userID] = append(t.activity[userID], activity)
	return activity.ID, nil
}

func (t *testImpl) ActivityForUser(userID types.UserID) ([]types.Activity, error) {
	return t.activity[userID], nil
}

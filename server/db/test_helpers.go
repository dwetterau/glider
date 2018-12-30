package db

import (
	"math/rand"

	"github.com/dwetterau/glider/server/types"
)

type testImpl struct {
	users    map[string]types.UserID
	activity map[types.UserID][]types.Activity
}

var _ Database = &testImpl{}

func TestOnlyMockImpl() Database {
	return &testImpl{
		users:    make(map[string]types.UserID),
		activity: make(map[types.UserID][]types.Activity),
	}
}
func (t *testImpl) AddUser(fbID string) (types.UserID, error) {
	if val, ok := t.users[fbID]; ok {
		return val, nil
	}
	userID := types.UserID(rand.Int63())
	t.users[fbID] = userID
	return userID, nil
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

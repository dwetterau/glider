package db

import (
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"glider/server/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initDB(t *testing.T) (Database, func()) {
	name, err := ioutil.TempDir("", "sqlite_test_dir")
	require.NoError(t, err)

	d, err := NewSQLite(path.Join(name, "new.db"))
	require.NoError(t, err)

	return d, func() {
		require.NoError(t, os.RemoveAll(name))
	}
}

func TestAddUser(t *testing.T) {
	d, toDefer := initDB(t)
	defer toDefer()

	id1, err := d.AddUser("test1")
	require.NoError(t, err)
	assert.Equal(t, id1, types.UserID(1))

	// Inserting it again should return the same id
	id1, err = d.AddUser("test1")
	require.NoError(t, err)
	assert.Equal(t, id1, types.UserID(1))

	// Now do it all again and expect userID 2 this time
	id2, err := d.AddUser("test2")
	require.NoError(t, err)
	assert.Equal(t, id2, types.UserID(2))

	// Inserting it again should return the same id
	id2, err = d.AddUser("test2")
	require.NoError(t, err)
	assert.Equal(t, id2, types.UserID(2))

	// Sanity check the first user still
	id1, err = d.AddUser("test1")
	require.NoError(t, err)
	assert.Equal(t, id1, types.UserID(1))
}

func TestAddActivities(t *testing.T) {
	d, toDefer := initDB(t)
	defer toDefer()

	userID1, err := d.AddUser("test1")
	require.NoError(t, err)

	userID2, err := d.AddUser("test2")
	require.NoError(t, err)

	utcTime := time.Unix(1239017850, 0)
	realTime := time.Unix(1231234195, 0)
	activities := []types.Activity{
		{ID: 1, Type: types.ActivityOverallDay, UTCDate: utcTime, ActualTime: realTime, Value: "val1"},
		{ID: 2, Type: types.ActivityUnknown, UTCDate: utcTime, ActualTime: realTime, Value: "val2"},
		{ID: 3, Type: types.ActivityOverallDay, UTCDate: utcTime, ActualTime: realTime, Value: "val3"},
	}

	activityID1, err := d.AddActivity(userID1, activities[0])
	require.NoError(t, err)
	assert.Equal(t, activities[0].ID, activityID1)

	activityID2, err := d.AddActivity(userID1, activities[1])
	require.NoError(t, err)
	assert.Equal(t, activities[1].ID, activityID2)

	activityID3, err := d.AddActivity(userID2, activities[2])
	require.NoError(t, err)
	assert.Equal(t, activities[2].ID, activityID3)

	// Now make sure we can fetch it all back out
	user1Activity, err := d.ActivityForUser(userID1)
	require.NoError(t, err)
	assert.Equal(t, activities[:2], user1Activity)

	user2Activity, err := d.ActivityForUser(userID2)
	require.NoError(t, err)
	assert.Equal(t, activities[2:], user2Activity)
}

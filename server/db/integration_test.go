package db

import (
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/dwetterau/glider/server/types"
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

	cali, err := time.LoadLocation("America/Los_Angeles")
	require.NoError(t, err)

	id1, tz1, err := d.AddUser("test1", time.UTC)
	require.NoError(t, err)
	assert.Equal(t, types.UserID(1), id1)
	assert.Equal(t, time.UTC.String(), tz1)

	// Inserting it again should return the same id
	id1, tz1, err = d.AddUser("test1", time.UTC)
	require.NoError(t, err)
	assert.Equal(t, types.UserID(1), id1)
	assert.Equal(t, time.UTC.String(), tz1)

	// Now do it all again and expect userID 2 this time
	id2, tz2, err := d.AddUser("test2", cali)
	require.NoError(t, err)
	assert.Equal(t, types.UserID(2), id2)
	assert.Equal(t, cali.String(), tz2)

	// Inserting it again should return the same id
	id2, tz2, err = d.AddUser("test2", cali)
	require.NoError(t, err)
	assert.Equal(t, types.UserID(2), id2)
	assert.Equal(t, cali.String(), tz2)

	// Sanity check the first user still
	id1, tz1, err = d.AddUser("test1", time.UTC)
	require.NoError(t, err)
	assert.Equal(t, types.UserID(1), id1)
	assert.Equal(t, time.UTC.String(), tz1)
}

func TestSetTimezone(t *testing.T) {
	d, toDefer := initDB(t)
	defer toDefer()

	id1, tz1, err := d.AddUser("test1", time.UTC)
	require.NoError(t, err)
	assert.Equal(t, time.UTC.String(), tz1)

	cali, err := time.LoadLocation("America/Los_Angeles")
	require.NoError(t, err)

	// Inserting it again should return the original timezone
	id1, tz1, err = d.AddUser("test1", cali)
	require.NoError(t, err)
	assert.Equal(t, time.UTC.String(), tz1)

	// Insert another user to be sure
	_, tz2, err := d.AddUser("test2", time.UTC)
	require.NoError(t, err)

	err = d.SetTimezone(id1, cali)
	require.NoError(t, err)

	// Now check the timezones again
	id1, tz1, err = d.AddUser("test1", cali)
	require.NoError(t, err)
	assert.Equal(t, cali.String(), tz1)

	_, tz2, err = d.AddUser("test2", cali)
	require.NoError(t, err)
	assert.Equal(t, time.UTC.String(), tz2)
}

func TestAddActivities(t *testing.T) {
	d, toDefer := initDB(t)
	defer toDefer()

	userID1, _, err := d.AddUser("test1", time.UTC)
	require.NoError(t, err)

	userID2, _, err := d.AddUser("test2", time.UTC)
	require.NoError(t, err)

	utcTime := time.Unix(1239017850, 0)
	realTime := time.Unix(1231234195, 0)
	activities := []types.Activity{
		{ID: 1, Type: types.ActivityOverallDay, UTCDate: utcTime, ActualTime: realTime, Value: "val1", RawValue: "v1"},
		{ID: 2, Type: types.ActivityUnknown, UTCDate: utcTime, ActualTime: realTime, Value: "val2", RawValue: "v2"},
		{ID: 3, Type: types.ActivityOverallDay, UTCDate: utcTime, ActualTime: realTime, Value: "val3", RawValue: "v3"},
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

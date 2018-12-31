package conversation

import (
	"testing"
	"time"

	"github.com/dwetterau/glider/server/db"
	"github.com/dwetterau/glider/server/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndToEnd(t *testing.T) {
	impl := &managerImpl{
		database:        db.TestOnlyMockImpl(),
		currentMessages: make(map[string]*state),
	}

	inputs := []string{
		"Start",
		"Overall",
		"AWESOME",
		"finished",
	}
	outputs := make([]string, 0, len(inputs))
	for _, input := range inputs {
		outputs = append(outputs, impl.Handle("fb1", input))
	}
	expectedOutputs := []string{
		"Hello! What type of activity do you want to record?",
		"How was your day?",
		"Great! If you're finished, feel free to say so, " +
			"otherwise let me know what type of activity you want to record.",
		"Have a nice day!",
	}
	assert.Equal(t, expectedOutputs, outputs)

	userID, _, err := impl.database.AddUser("fb1", time.UTC)
	require.NoError(t, err)
	activities, err := impl.database.ActivityForUser(userID)
	assert.Len(t, activities, 1)
	assert.Equal(t, activities[0].Type, types.ActivityOverallDay)
	assert.Equal(t, activities[0].Value, "great")
	assert.Equal(t, activities[0].RawValue, "AWESOME")
}

func TestSetTimezone(t *testing.T) {
	impl := &managerImpl{
		database:        db.TestOnlyMockImpl(),
		currentMessages: make(map[string]*state),
	}

	inputs := []string{
		"Start",
		"TIMEZONE",
		"America/Los_Angeles",
		"finished",
	}
	outputs := make([]string, 0, len(inputs))
	for _, input := range inputs {
		outputs = append(outputs, impl.Handle("fb1", input))
	}
	expectedOutputs := []string{
		"Hello! What type of activity do you want to record?",
		"Your current timezone is UTC. What would you like to change it to?",
		"Thanks! Now what kind of activity would you like to record?",
		"I don't have anything to record, but have a nice day!",
	}
	assert.Equal(t, expectedOutputs, outputs)

	_, tz, err := impl.database.AddUser("fb1", time.UTC)
	require.NoError(t, err)
	assert.Equal(t, "America/Los_Angeles", tz.String())
}

package conversation

import (
	"testing"
	"time"

	"github.com/dwetterau/glider/server/db"
	"github.com/dwetterau/glider/server/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOverallDay(t *testing.T) {
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
		"I finished writing that down, what activity type would you like to record next?",
		"Have a nice day!",
	}
	assert.Equal(t, expectedOutputs, outputs)

	userID, _, err := impl.database.AddOrGetUser("fb1", time.UTC)
	require.NoError(t, err)
	activities, err := impl.database.ActivityForUser(userID)
	assert.Len(t, activities, 1)
	assert.Equal(t, types.ActivityOverallDay, activities[0].Type)
	assert.Equal(t, "great", activities[0].Value)
	assert.Equal(t, "AWESOME", activities[0].RawMessages)
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
		"Have a nice day!",
	}
	assert.Equal(t, expectedOutputs, outputs)

	_, tz, err := impl.database.AddOrGetUser("fb1", time.UTC)
	require.NoError(t, err)
	assert.Equal(t, "America/Los_Angeles", tz.String())
}

func TestProgramming(t *testing.T) {
	impl := &managerImpl{
		database:        db.TestOnlyMockImpl(),
		currentMessages: make(map[string]*state),
	}

	inputs := []string{
		"Start",
		"wrote CODE",
		"10m",
		"meh",
	}
	outputs := make([]string, 0, len(inputs))
	for _, input := range inputs {
		outputs = append(outputs, impl.Handle("fb1", input))
	}
	expectedOutputs := []string{
		"Hello! What type of activity do you want to record?",
		"How long did you program for?",
		"Okay, and how did you feel about that?",
		"I finished writing that down, what activity type would you like to record next?",
	}
	assert.Equal(t, expectedOutputs, outputs)

	userID, _, err := impl.database.AddOrGetUser("fb1", time.UTC)
	require.NoError(t, err)
	activities, err := impl.database.ActivityForUser(userID)
	assert.Len(t, activities, 1)
	assert.Equal(t, types.ActivityProgramming, activities[0].Type)
	assert.Equal(t, 10*time.Minute, activities[0].Duration)
	assert.Equal(t, "neutral", activities[0].Value)
	assert.Equal(t, "10m\nmeh", activities[0].RawMessages)
}

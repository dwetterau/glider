package conversation

import (
	"strings"
	"testing"
	"time"

	"github.com/dwetterau/glider/server/db"
	"github.com/dwetterau/glider/server/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	witai "github.com/wit-ai/wit-go"
)

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
		newUserWelcomeMessage,
		"Your current timezone is UTC. What would you like to change it to?",
		"Thanks! Now what kind of activity would you like to record?",
		"Have a nice day!",
	}
	assert.Equal(t, expectedOutputs, outputs)

	_, tz, _, err := impl.database.AddOrGetUser("fb1", time.UTC)
	require.NoError(t, err)
	assert.Equal(t, "America/Los_Angeles", tz.String())
}

func TestSummary(t *testing.T) {
	impl := &managerImpl{
		database:        db.TestOnlyMockImpl(),
		currentMessages: make(map[string]*state),
	}
	userID, _, _, err := impl.database.AddOrGetUser("fb1", time.UTC)
	require.NoError(t, err)

	_, utcDate := nowAndUTCDate(time.UTC)
	sameTime := utcDate.Local()
	activities := []types.Activity{
		{Type: types.ActivityClimbing, Duration: time.Hour, Value: "good", UTCDate: utcDate},
		{Type: types.ActivityOverallDay, Value: "great", UTCDate: sameTime},
		// This would should be excluded
		{Type: types.ActivityYoga, Duration: time.Hour, Value: "great", UTCDate: utcDate.Add(time.Second)},
	}
	for _, activity := range activities {
		_, err = impl.database.AddOrUpdateActivity(userID, activity)
		require.NoError(t, err)
	}
	inputs := []string{
		"Start",
		"summary",
	}
	outputs := make([]string, 0, len(inputs))
	for _, input := range inputs {
		outputs = append(outputs, impl.Handle("fb1", input))
	}
	expectedOutputs := []string{
		"Welcome back! What activity do you want to record?",
		"Today you've recorded that:\n\n" +
			"-  Your day was great.\n" +
			"-  You climbed for 1h and felt good about it.",
	}
	assert.Equal(t, expectedOutputs, outputs)
}

type mockWitClient struct {
	resp witai.MessageResponse
}

var _ WitClient = &mockWitClient{}

func (m *mockWitClient) Parse(req *witai.MessageRequest) (*witai.MessageResponse, error) {
	return &m.resp, nil
}

func runTestWithWit(
	t *testing.T,
	witResp witai.MessageResponse,
	inputs []string,
	expectedOutputs []string,
	expectedActivity types.Activity,
) {
	impl := &managerImpl{
		database:        db.TestOnlyMockImpl(),
		witClient:       &mockWitClient{resp: witResp},
		currentMessages: make(map[string]*state),
	}
	outputs := make([]string, 0, len(inputs))
	for _, input := range inputs {
		outputs = append(outputs, impl.Handle("fb1", input))
	}
	assert.Equal(t, expectedOutputs, outputs)
	userID, _, _, err := impl.database.AddOrGetUser("fb1", time.UTC)
	require.NoError(t, err)
	activities, err := impl.database.ActivityForUser(userID)
	assert.Len(t, activities, 1)

	// Clear this out prior to the comparison (it's random, or non-deterministic)
	activities[0].ID = 0
	activities[0].ActualTime = time.Time{}
	activities[0].UTCDate = time.Time{}

	assert.Equal(t, expectedActivity, activities[0])
}

func runTest(
	t *testing.T,
	inputs []string,
	expectedOutputs []string,
	expectedActivity types.Activity,
) {
	runTestWithWit(t, witai.MessageResponse{}, inputs, expectedOutputs, expectedActivity)
}

func TestMulti(t *testing.T) {
	impl := &managerImpl{
		database:        db.TestOnlyMockImpl(),
		witClient:       &mockWitClient{},
		currentMessages: make(map[string]*state),
	}

	inputs := []string{
		"Start",
		"day",
		"GREAT",
		"yoga",
		"2h",
		"good",
		"finished",
	}
	outputs := make([]string, 0, len(inputs))
	for _, input := range inputs {
		outputs = append(outputs, impl.Handle("fb1", input))
	}
	expectedOutputs := []string{
		newUserWelcomeMessage,
		"How was your day?",
		"I finished writing that down, what activity type would you like to record next?",
		"How long did you do yoga for?",
		"Okay, and how did you feel about that?",
		"I finished writing that down, what activity type would you like to record next?",
		"Have a nice day!",
	}
	assert.Equal(t, expectedOutputs, outputs)

	// Make sure both activities were recorded
	userID, _, _, err := impl.database.AddOrGetUser("fb1", time.UTC)
	require.NoError(t, err)
	activities, err := impl.database.ActivityForUser(userID)
	assert.Len(t, activities, 2)
}

func TestOverallDay(t *testing.T) {
	inputs := []string{
		"Start",
		"Overall",
		"AWESOME",
		"finished",
	}
	expectedOutputs := []string{
		newUserWelcomeMessage,
		"How was your day?",
		"I finished writing that down, what activity type would you like to record next?",
		"Have a nice day!",
	}
	runTest(t, inputs, expectedOutputs, types.Activity{
		Type:        types.ActivityOverallDay,
		Value:       "great",
		RawMessages: "AWESOME",
	})
}

func TestProgramming(t *testing.T) {
	inputs := []string{
		"Start",
		"wrote CODE",
		"10h",
		"meh",
	}
	expectedOutputs := []string{
		newUserWelcomeMessage,
		"How long did you program for?",
		"Okay, and how did you feel about that?",
		"I finished writing that down, what activity type would you like to record next?",
	}
	activity := types.Activity{
		Type:        types.ActivityProgramming,
		Duration:    10 * time.Hour,
		Value:       "neutral",
		RawMessages: strings.Join(inputs[2:], "\n"),
	}
	runTest(t, inputs, expectedOutputs, activity)

	// Now do it fast this time.
	witResponse := loadTestData(t)[5].resp
	inputs = []string{inputs[0], inputs[1], inputs[3]}
	expectedOutputs = []string{expectedOutputs[0], expectedOutputs[2], expectedOutputs[3]}
	activity.RawMessages = "wrote CODE\nmeh"
	runTestWithWit(t, witResponse, inputs, expectedOutputs, activity)
}

func TestLaundry(t *testing.T) {
	inputs := []string{
		"Start",
		"LAUNDRY",
		"2",
		"great",
	}
	expectedOutputs := []string{
		newUserWelcomeMessage,
		"How many loads of laundry did you do?",
		"Okay, and how did you feel about that?",
		"I finished writing that down, what activity type would you like to record next?",
	}
	activity := types.Activity{
		Type:        types.ActivityLaundry,
		Count:       2,
		Value:       "great",
		RawMessages: strings.Join(inputs[2:], "\n"),
	}
	runTest(t, inputs, expectedOutputs, activity)

	// Now do it fast this time.
	witResponse := loadTestData(t)[10].resp
	activity.RawMessages = "LAUNDRY\ngreat"
	inputs = []string{inputs[0], inputs[1], inputs[3]}
	expectedOutputs = []string{expectedOutputs[0], expectedOutputs[2], expectedOutputs[3]}
	runTestWithWit(t, witResponse, inputs, expectedOutputs, activity)
}

func TestRunning(t *testing.T) {
	inputs := []string{
		"Start",
		"running",
		"5",
		"30m",
		"great",
	}
	expectedOutputs := []string{
		newUserWelcomeMessage,
		"How far did you run in miles?",
		"How long did you run for?",
		"Okay, and how did you feel about that?",
		"I finished writing that down, what activity type would you like to record next?",
	}
	activity := types.Activity{
		Type:        types.ActivityRunning,
		Count:       5,
		Duration:    30 * time.Minute,
		Value:       "great",
		RawMessages: strings.Join(inputs[2:], "\n"),
	}
	runTest(t, inputs, expectedOutputs, activity)

	// Now do it fast this time.
	witResponse := loadTestData(t)[2].resp
	inputs = []string{inputs[0], inputs[1], inputs[4]}
	expectedOutputs = []string{expectedOutputs[0], expectedOutputs[3], expectedOutputs[4]}
	activity.RawMessages = "running\ngreat"
	runTestWithWit(t, witResponse, inputs, expectedOutputs, activity)
}

func TestMeetings(t *testing.T) {
	inputs := []string{
		"Start",
		"meeting",
		"4",
		"5h",
		"great",
	}
	expectedOutputs := []string{
		newUserWelcomeMessage,
		"How many meetings did you go to?",
		"What was the total time you spent in meetings?",
		"Okay, and how did you feel about that?",
		"I finished writing that down, what activity type would you like to record next?",
	}
	activity := types.Activity{
		Type:        types.ActivityMeetings,
		Count:       4,
		Duration:    5 * time.Hour,
		Value:       "great",
		RawMessages: strings.Join(inputs[2:], "\n"),
	}
	runTest(t, inputs, expectedOutputs, activity)

	// Now do it fast this time.
	witResponse := loadTestData(t)[6].resp
	inputs = []string{inputs[0], inputs[1], inputs[4]}
	expectedOutputs = []string{expectedOutputs[0], expectedOutputs[3], expectedOutputs[4]}
	activity.RawMessages = "meeting\ngreat"
	runTestWithWit(t, witResponse, inputs, expectedOutputs, activity)
}

func TestReading(t *testing.T) {
	inputs := []string{
		"Start",
		"reading",
		"100",
		"2h",
		"great",
	}
	expectedOutputs := []string{
		newUserWelcomeMessage,
		"How many pages did you read?",
		"How long did you read for?",
		"Okay, and how did you feel about that?",
		"I finished writing that down, what activity type would you like to record next?",
	}
	activity := types.Activity{
		Type:        types.ActivityReading,
		Count:       100,
		Duration:    2 * time.Hour,
		Value:       "great",
		RawMessages: strings.Join(inputs[2:], "\n"),
	}
	runTest(t, inputs, expectedOutputs, activity)

	// Now do it fast this time.
	witResponse := loadTestData(t)[12].resp
	inputs = []string{inputs[0], inputs[1], inputs[4]}
	expectedOutputs = []string{expectedOutputs[0], expectedOutputs[3], expectedOutputs[4]}
	activity.RawMessages = "reading\ngreat"
	runTestWithWit(t, witResponse, inputs, expectedOutputs, activity)
}

func TestClimbing(t *testing.T) {
	inputs := []string{
		"Start",
		"climbing",
		"2h",
		"great",
	}
	expectedOutputs := []string{
		newUserWelcomeMessage,
		"How long did you climb for?",
		"Okay, and how did you feel about that?",
		"I finished writing that down, what activity type would you like to record next?",
	}
	activity := types.Activity{
		Type:        types.ActivityClimbing,
		Duration:    2 * time.Hour,
		Value:       "great",
		RawMessages: strings.Join(inputs[2:], "\n"),
	}
	runTest(t, inputs, expectedOutputs, activity)

	// Now do it fast this time.
	witResponse := loadTestData(t)[0].resp
	inputs = []string{inputs[0], inputs[1], inputs[3]}
	expectedOutputs = []string{expectedOutputs[0], expectedOutputs[2], expectedOutputs[3]}
	activity.RawMessages = "climbing\ngreat"
	runTestWithWit(t, witResponse, inputs, expectedOutputs, activity)
}

func TestYoga(t *testing.T) {
	inputs := []string{
		"Start",
		"yoga",
		"1h",
		"great",
	}
	expectedOutputs := []string{
		newUserWelcomeMessage,
		"How long did you do yoga for?",
		"Okay, and how did you feel about that?",
		"I finished writing that down, what activity type would you like to record next?",
	}
	activity := types.Activity{
		Type:        types.ActivityYoga,
		Duration:    time.Hour,
		Value:       "great",
		RawMessages: strings.Join(inputs[2:], "\n"),
	}
	runTest(t, inputs, expectedOutputs, activity)

	// Now do it fast this time.
	witResponse := loadTestData(t)[13].resp
	inputs = []string{inputs[0], inputs[1], inputs[3]}
	expectedOutputs = []string{expectedOutputs[0], expectedOutputs[2], expectedOutputs[3]}
	activity.RawMessages = "yoga\ngreat"
	runTestWithWit(t, witResponse, inputs, expectedOutputs, activity)
}

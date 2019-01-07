package conversation

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dwetterau/glider/server/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wit-ai/wit-go"
)

type messageAndResponse struct {
	message string
	resp    witai.MessageResponse
}

func loadTestData(t *testing.T) []messageAndResponse {
	_, filename, _, _ := runtime.Caller(0)
	parentPath := filepath.Join(filepath.Dir(filename), "wit", "wit_output.json")
	allOutput, err := ioutil.ReadFile(parentPath)
	require.NoError(t, err)

	var responses []witai.MessageResponse
	err = json.Unmarshal(allOutput, &responses)
	require.NoError(t, err)

	data := make([]messageAndResponse, len(TestMessages))
	for i, msg := range TestMessages {
		data[i].message = msg
		data[i].resp = responses[i]
	}
	return data
}

func parseData(t *testing.T, index int) parsedWitMessage {
	data := loadTestData(t)[index].resp
	parser := determineWitParser(data)
	return parser(data)
}

func TestParseProgramming(t *testing.T) {
	parsed := parseData(t, 5)
	assert.Equal(t, parsed.statesToSkip, map[stateType]struct{}{
		askingActivityDuration: {},
	})
	assert.Equal(t, parsed.newActivity, &types.Activity{
		Type:     types.ActivityProgramming,
		Duration: 10 * time.Hour,
	})
}

func TestParseLaundry(t *testing.T) {
	parsed := parseData(t, 10)
	assert.Equal(t, parsed.statesToSkip, map[stateType]struct{}{askingActivityCount: {}})
	assert.Equal(t, parsed.newActivity, &types.Activity{
		Type:  types.ActivityLaundry,
		Count: 2,
	})
}

func TestParseRunning(t *testing.T) {
	parsed := parseData(t, 3)
	assert.Equal(t, parsed.statesToSkip, map[stateType]struct{}{
		askingActivityCount:    {},
		askingActivityDuration: {},
	})
	assert.Equal(t, parsed.newActivity, &types.Activity{
		Type:     types.ActivityRunning,
		Duration: 28 * time.Minute,
		Count:    4,
	})
}

func TestParseRunningWithDateAdjustment(t *testing.T) {
	parsed := parseData(t, 2)
	assert.Equal(t, parsed.statesToSkip, map[stateType]struct{}{})
	assert.NotNil(t, parsed.desiredTime)
	assert.Equal(t, parsed.newActivity, &types.Activity{
		Type: types.ActivityRunning,
	})
}

func TestParseMeetings(t *testing.T) {
	parsed := parseData(t, 6)
	assert.Equal(t, parsed.statesToSkip, map[stateType]struct{}{askingActivityDuration: {}, askingActivityCount: {}})
	assert.Equal(t, parsed.newActivity, &types.Activity{
		Type:     types.ActivityMeetings,
		Duration: 5 * time.Hour,
		Count:    4,
	})
}

func TestParseReading(t *testing.T) {
	parsed := parseData(t, 12)
	assert.Equal(t, parsed.statesToSkip, map[stateType]struct{}{askingActivityDuration: {}, askingActivityCount: {}})
	assert.Equal(t, parsed.newActivity, &types.Activity{
		Type:     types.ActivityReading,
		Duration: 2 * time.Hour,
		Count:    100,
	})
}

func TestParseYoga(t *testing.T) {
	parsed := parseData(t, 13)
	assert.Equal(t, parsed.statesToSkip, map[stateType]struct{}{askingActivityDuration: {}})
	assert.Equal(t, parsed.newActivity, &types.Activity{
		Type:     types.ActivityYoga,
		Duration: time.Hour,
	})
}

func TestParseClimbing(t *testing.T) {
	parsed := parseData(t, 0)
	assert.Equal(t, parsed.statesToSkip, map[stateType]struct{}{askingActivityDuration: {}})
	assert.Equal(t, parsed.newActivity, &types.Activity{
		Type:     types.ActivityClimbing,
		Duration: 2 * time.Hour,
	})
}

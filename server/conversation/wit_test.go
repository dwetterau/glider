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
	parsed := parseData(t, 15)
	assert.Equal(t, parsed.statesToSkip, map[stateType]struct{}{
		askingActivityDuration: {},
	})
	assert.Equal(t, parsed.newActivity, &types.Activity{
		Type:     types.ActivityProgramming,
		Duration: 10 * time.Hour,
	})
}

func TestParseRunning(t *testing.T) {
	parsed := parseData(t, 8)
	assert.Equal(t, parsed.statesToSkip, map[stateType]struct{}{
		askingActivityCount:    {},
		askingActivityDuration: {},
	})
	assert.Equal(t, parsed.newActivity, &types.Activity{
		Type:     types.ActivityRunning,
		Duration: 30 * time.Minute,
		Count:    5,
	})
}

func TestParseClimbing(t *testing.T) {
	parsed := parseData(t, 7)
	assert.Equal(t, parsed.statesToSkip, map[stateType]struct{}{askingActivityDuration: {}})
	assert.Equal(t, parsed.newActivity, &types.Activity{
		Type:     types.ActivityClimbing,
		Duration: 2 * time.Hour,
	})
}

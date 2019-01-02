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

func TestParseClimbing(t *testing.T) {
	climbingData := loadTestData(t)[7].resp
	parser := determineWitParser(climbingData)
	parsed, errorMessage := parser(climbingData)
	assert.Equal(t, errorMessage, "")
	assert.Equal(t, parsed.statesToSkip, map[stateType]struct{}{askingActivityDuration: {}})
	assert.Equal(t, parsed.newActivity, &types.Activity{
		Type:     types.ActivityClimbing,
		Duration: 2 * time.Hour,
	})
}

func TestParseRunning(t *testing.T) {
	data := loadTestData(t)[8].resp
	parser := determineWitParser(data)
	parsed, errorMessage := parser(data)
	assert.Equal(t, errorMessage, "")
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

package conversation

import (
	"fmt"
	"math"
	"time"

	"github.com/dwetterau/glider/server/types"
	"github.com/wit-ai/wit-go"
)

type WitClient interface {
	Parse(req *witai.MessageRequest) (*witai.MessageResponse, error)
}

var _ WitClient = &witai.Client{}

type parsedWitMessage struct {
	newActivity  *types.Activity
	statesToSkip map[stateType]struct{}
}

func parseMessage(client WitClient, message string) (parsedWitMessage, string) {
	response, err := client.Parse(&witai.MessageRequest{Query: message})
	if err != nil {
		fmt.Println("got an error from Wit.ai:", err)
		return parsedWitMessage{}, "Sorry, there was an error parsing your message."
	}

	f := determineWitParser(*response)
	if f == nil {
		// This will be interpretted as "can't figure out the type of activity"
		return parsedWitMessage{}, ""
	}
	return f(*response)
}

func determineWitParser(response witai.MessageResponse) func(witai.MessageResponse) (parsedWitMessage, string) {
	for entityName := range response.Entities {
		if entityName == "running" {
			return runningWitParser
		}
		if entityName == "climbing" {
			return climbingWitParser
		}
	}
	// TODO: Add more parsers
	return nil
}

func runningWitParser(response witai.MessageResponse) (parsedWitMessage, string) {
	parsedMessage := parsedWitMessage{
		newActivity: &types.Activity{
			Type: types.ActivityRunning,
		},
		statesToSkip: make(map[stateType]struct{}),
	}
	// See if there's a duration too
	for name, entity := range response.Entities {
		if name == "duration" {
			duration := parseDuration(entity)
			if duration != nil {
				parsedMessage.statesToSkip[askingActivityDuration] = struct{}{}
				parsedMessage.newActivity.Duration = *duration
			}
		}
		if name == "distance" {
			distance := parseDistance(entity)
			if distance != nil {
				parsedMessage.statesToSkip[askingActivityCount] = struct{}{}
				parsedMessage.newActivity.Count = *distance
			}
		}
	}
	return parsedMessage, ""
}

func climbingWitParser(response witai.MessageResponse) (parsedWitMessage, string) {
	parsedMessage := parsedWitMessage{
		newActivity: &types.Activity{
			Type: types.ActivityClimbing,
		},
		statesToSkip: make(map[stateType]struct{}),
	}
	// See if there's a duration too
	for name, entity := range response.Entities {
		if name == "duration" {
			duration := parseDuration(entity)
			if duration != nil {
				parsedMessage.statesToSkip[askingActivityDuration] = struct{}{}
				parsedMessage.newActivity.Duration = *duration
			}
		}
	}
	return parsedMessage, ""
}

func parseDistance(entity interface{}) *int64 {
	entityList, ok := entity.([]interface{})
	if !ok {
		return nil
	}
	if len(entityList) != 1 {
		return nil
	}
	entity = entityList[0]
	valueMap, ok := entity.(map[string]interface{})
	if !ok {
		return nil
	}
	// TODO: This should look at the units.
	valInterface, ok := valueMap["value"]
	if !ok {
		return nil
	}
	val, ok := valInterface.(float64)
	if !ok {
		return nil
	}
	local := int64(math.Round(val))
	return &local
}

func parseDuration(entity interface{}) *time.Duration {
	entityList, ok := entity.([]interface{})
	if !ok {
		return nil
	}
	if len(entityList) != 1 {
		return nil
	}
	entity = entityList[0]
	normalizedMap, ok := entity.(map[string]interface{})
	if !ok {
		return nil
	}
	valueMapInterface, ok := normalizedMap["normalized"]
	if !ok {
		return nil
	}
	valueMap, ok := valueMapInterface.(map[string]interface{})
	if !ok {
		return nil
	}
	durationStringSeconds, ok := valueMap["value"]
	if !ok {
		return nil
	}
	durationFloatSeconds, ok := durationStringSeconds.(float64)
	if !ok {
		return nil
	}
	local := time.Duration(math.Round(durationFloatSeconds)) * time.Second
	return &local
}

var TestMessages = []string{
	"Paula and I went on a 10 mile hike for 4 hours",
	"Adam and I went to a movie",
	"Had a meeting with Braden",
	"Met with Bill",
	"Did 6 loads of laundry",
	"Read 100 pages in 2 hours",
	"Did yoga for an hour",
	"Went climbing for 2 hours",
	"Ran 5 miles in 30 minutes",
	"Ran 4 miles in 28 minutes",
	"Went on a 4 mile run",
	"Walked to the store",
	"Watched a movie with Sam",
	"Played Fortnite with Sam and Braden",
	"Ate lunch with Adam",
	"Programmed for 10 hours",
	"Climbed for 2 hours",
	"Did 2 loads of laundry",
}

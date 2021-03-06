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
	desiredTime  *time.Time
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
	return f(*response), ""
}

func determineWitParser(response witai.MessageResponse) func(witai.MessageResponse) parsedWitMessage {
	for entityName := range response.Entities {
		if entityName == "programming" {
			return genericParser(types.ActivityProgramming, map[string]struct{}{"duration": {}})
		}
		if entityName == "laundry" {
			return genericParser(types.ActivityLaundry, map[string]struct{}{"loads": {}})
		}
		if entityName == "running" {
			return genericParser(types.ActivityRunning, map[string]struct{}{"duration": {}, "distance": {}})
		}
		if entityName == "meeting" {
			return genericParser(types.ActivityMeetings, map[string]struct{}{"duration": {}, "meetings": {}})
		}
		if entityName == "reading" {
			return genericParser(types.ActivityReading, map[string]struct{}{"duration": {}, "pages": {}})
		}
		if entityName == "yoga" {
			return genericParser(types.ActivityYoga, map[string]struct{}{"duration": {}})
		}
		if entityName == "climbing" {
			return genericParser(types.ActivityClimbing, map[string]struct{}{"duration": {}})
		}
	}
	// TODO: Add more parsers
	return nil
}

var numberParserNames = map[string]struct{}{
	"loads":    {},
	"pages":    {},
	"meetings": {},
}

func genericParser(
	activityType types.ActivityType,
	namesToParse map[string]struct{},
) func(witai.MessageResponse) parsedWitMessage {
	return func(response witai.MessageResponse) parsedWitMessage {
		parsedMessage := parsedWitMessage{
			newActivity: &types.Activity{
				Type: activityType,
			},
			statesToSkip: make(map[stateType]struct{}),
		}
		for name, entity := range response.Entities {
			if name == "datetime" {
				parsedMessage.desiredTime = parseDatetime(entity)
			}
			if _, ok := namesToParse[name]; !ok {
				continue
			}
			if name == "duration" {
				duration := parseDuration(entity)
				if duration != nil {
					parsedMessage.statesToSkip[askingActivityDuration] = struct{}{}
					parsedMessage.newActivity.Duration = *duration
					continue
				}
			}
			if name == "distance" {
				distance := parseDistance(entity)
				if distance != nil {
					parsedMessage.statesToSkip[askingActivityCount] = struct{}{}
					parsedMessage.newActivity.Count = *distance
					continue
				}
			}
			if _, ok := numberParserNames[name]; ok {
				number := parseNumber(entity)
				if number != nil {
					parsedMessage.statesToSkip[askingActivityCount] = struct{}{}
					parsedMessage.newActivity.Count = *number
					continue
				}
			}
		}
		return parsedMessage
	}
}

func parseNumber(entity interface{}) *int64 {
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

func parseDistance(entity interface{}) *int64 {
	// TODO: This should look at the units.
	return parseNumber(entity)
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

func parseDatetime(entity interface{}) *time.Time {
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
	valInterface, ok := valueMap["value"]
	if !ok {
		return nil
	}
	val, ok := valInterface.(string)
	if !ok {
		return nil
	}
	local, err := time.Parse("2006-01-02T15:04:05.999-07:00", val)
	if err != nil {
		return nil
	}
	return &local
}

var TestMessages = []string{
	"Went climbing for 2 hours",
	"Climbed for 2 hours",

	"Ran 5 miles in 30 minutes",
	"4 mile run for 28 minutes",
	"Went on a 4 mile run",

	"Programmed for 10 hours",

	"Went to 4 meetings for 5 hours",
	"Met for 3 hours 6 times",
	"Had a meeting with Braden",
	"Met with Bill",

	"Did 2 loads of laundry",
	"Did 6 loads of laundry",

	"Read 100 pages for 2 hours",

	"Did yoga for an hour",

	// Not yet categorized:
	"Walked to the store",
	"Watched a movie with Sam",
	"Ate lunch with Adam",
	"Adam and I went to a movie",
	"Paula and I went on a 10 mile hike for 4 hours",
	"Played Fortnite with Sam and Braden",
}

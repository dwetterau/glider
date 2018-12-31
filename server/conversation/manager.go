package conversation

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dwetterau/glider/server/db"
	"github.com/dwetterau/glider/server/types"
)

const messageTimeout = 5 * time.Minute

type state struct {
	startTime    time.Time
	lastMessage  time.Time
	currentState stateType

	currentActivityType types.ActivityType
	activity            *types.Activity

	// Initialized on start
	userID       types.UserID
	userTimezone *time.Location
}

type stateType int

const (
	unknownStateType stateType = iota
	askingActivityType
	askingActivityValue
	askingActivityDuration
	askingActivityCount
	askingTimezone
)

var activityValues = map[stateType]struct{}{
	askingActivityValue:    {},
	askingActivityDuration: {},
	askingActivityCount:    {},
}

// State machine:
//
//                +----------------------------+
//                v                            |
// start -> askingActivityType ------> askingActivityValue
//             |     ^
//             v     |
//          askingTimezone

type managerImpl struct {
	database        db.Database
	currentMessages map[string]*state
	lock            sync.RWMutex
}

var _ Manager = &managerImpl{}

// Starts a goroutine to keep the map pruned
func (m *managerImpl) start() {
	go func() {
		for {
			m.lock.Lock()
			// See if any conversations are too old
			var toRemove []string
			for id, s := range m.currentMessages {
				if time.Since(s.lastMessage) > messageTimeout {
					toRemove = append(toRemove, id)
				}
			}
			for _, id := range toRemove {
				delete(m.currentMessages, id)
			}
			m.lock.Unlock()
			time.Sleep(messageTimeout / 2)
		}
	}()
}

func (m *managerImpl) Handle(fbID string, message string) string {
	message = strings.Replace(message, "\n", " ", -1)
	command := strings.ToLower(message)
	if command == "help" {
		return "Say \"start\" to begin. " +
			"Say \"finished\" when you're done. " +
			"At any point feel free to \"quit\"."
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	curState, ok := m.currentMessages[fbID]
	if !ok {
		if command != "start" && command != "hello" && command != "hi" {
			return "Sorry, your conversation might have timed out. Please start again."
		}
		// Load the user's information
		userID, timezone, err := m.database.AddOrGetUser(fbID, time.UTC)
		if err != nil {
			log.Println("Error loading user: ", err.Error())
			return "Sorry, I can't handle new conversations at this time. Try again shortly."
		}

		// Start a new message!
		m.currentMessages[fbID] = &state{
			startTime:    time.Now(),
			lastMessage:  time.Now(),
			currentState: askingActivityType,
			userID:       userID,
			userTimezone: timezone,
		}
		return "Hello! What type of activity do you want to record?"
	}
	// We already have a conversation going on, check for a few commands
	if command == "quit" || command == "abort" || command == "done" || command == "finished" || command == "stop" {
		delete(m.currentMessages, fbID)
		return "Have a nice day!"
	}

	// Finally, update the activity however the user wants to.
	if curState.currentState == askingActivityType {
		if command == "timezone" {
			curState.currentState = askingTimezone
			return fmt.Sprintf(
				"Your current timezone is %s. What would you like to change it to?",
				curState.userTimezone,
			)
		}

		activityType, startMessage, startState := determineActivityType(command)
		if activityType == types.ActivityUnknown {
			return "Sorry, I don't know what type of activity that is. " +
				"Try saying something like \"overall\"."
		}
		curState.currentActivityType = activityType
		curState.currentState = startState
		return startMessage
	} else if _, ok := activityValues[curState.currentState]; ok {
		activity, nextState, response := handleResponse(
			curState.currentActivityType,
			curState.currentState,
			command,
		)
		if activity == nil {
			return response
		}

		if curState.activity == nil {
			// Initialize the activity
			now, utcDate := nowAndUTCDate(curState.userTimezone)
			activity.Type = curState.currentActivityType
			activity.UTCDate = utcDate
			activity.ActualTime = now
			activity.RawMessages = message
			curState.activity = activity
		} else {
			// Append another message
			curState.activity.RawMessages += "\n" + message
		}

		// Copy over fields based off the current thing we asked the user about
		if curState.currentState == askingActivityValue {
			curState.activity.Value = activity.Value
		} else if curState.currentState == askingActivityCount {
			curState.activity.Count = activity.Count
		} else if curState.currentState == askingActivityDuration {
			curState.activity.Duration = activity.Duration
		} else {
			return "Sorry, the programmer messed this up. Please let them know."
		}

		// If this is the next state, we must be done!
		if nextState == askingActivityType {
			// Save the messages!
			// Note: This timezone value can always be changed later.
			userID, _, err := m.database.AddOrGetUser(fbID, time.UTC)
			if err != nil {
				log.Println("Error adding user: ", err.Error())
				return "Whoops, there was a problem saving your activity, try again shortly."
			}
			_, err = m.database.AddOrUpdateActivity(userID, *curState.activity)
			if err != nil {
				log.Println("Error saving activity: ", err.Error())
				return "Whoops, there was a problem saving your activity, try again shortly."
			}
			curState.activity = nil
			return "I finished writing that down, what activity type would you like to record next?"
		}

		// Otherwise, go to the next state.
		curState.currentState = nextState
		return response
	} else if curState.currentState == askingTimezone {
		// Try to parse the timezone
		tz, err := time.LoadLocation(message)
		if err != nil {
			log.Println("Error setting timezone: ", err.Error())
			return "Sorry, I don't know what timezone that is. Please try again."
		}

		// Now save the timezone
		err = m.database.SetTimezone(curState.userID, tz)
		if err != nil {
			log.Println("Error saving timezone: ", err.Error())
			return "Whoops, there was a problem saving your timezone, try again shortly."
		}
		curState.userTimezone = tz
		curState.currentState = askingActivityType
		return "Thanks! Now what kind of activity would you like to record?"
	}
	return "Sorry, I can't understand what you're saying. You can say \"help\" for some help getting started."
}

func nowAndUTCDate(userTimezone *time.Location) (time.Time, time.Time) {
	now := time.Now().In(userTimezone)
	year, month, day := now.Date()
	utcDate, err := time.Parse("2006-01-02", fmt.Sprintf("%d-%d-%d", year, month, day))
	if err != nil {
		panic(err)
	}
	return now, utcDate
}

func determineActivityType(command string) (types.ActivityType, string, stateType) {
	if command == "overall" || command == "overall day" || command == "day" {
		return types.ActivityOverallDay, "How was your day?", askingActivityValue
	}
	if command == "programming" || command == "programmed" || command == "wrote code" || command == "coded" {
		return types.ActivityProgramming, "How long did you program for?", askingActivityDuration
	}
	if command == "laundry" {
		return types.ActivityLaundry, "How many loads of laundry did you do?", askingActivityCount
	}
	return types.ActivityUnknown, "", unknownStateType
}

type successAndNextState struct {
	successMessage string
	nextState      stateType
}

// Some common helpers for the overall handler map.
var sentiment = successAndNextState{
	successMessage: "Okay, and how did you feel about that?",
	nextState:      askingActivityValue,
}

var done = successAndNextState{
	successMessage: "",
	nextState:      askingActivityType,
}

var handlerMap = map[types.ActivityType]map[stateType]successAndNextState{
	types.ActivityOverallDay: {
		askingActivityValue: done,
	},
	types.ActivityProgramming: {
		askingActivityDuration: sentiment,
		askingActivityValue:    done,
	},
	types.ActivityLaundry: {
		askingActivityCount: sentiment,
		askingActivityValue: done,
	},
}

// Handlers here must fill in the value fields, everything else is handled above this layer.
//
// Returns: The new activity struct (or nil), the next state (might be the same one) a message (might be an error).
func handleResponse(
	activityType types.ActivityType,
	currentState stateType,
	command string,
) (*types.Activity, stateType, string) {
	next, ok := handlerMap[activityType][currentState]
	if !ok {
		return nil, unknownStateType, "Sorry, the programmer messed this up. Please let them know."
	}
	if currentState == askingActivityDuration {
		d, err := time.ParseDuration(command)
		if err != nil {
			return nil, askingActivityDuration, "Sorry, I can't understand that duration value."
		}
		return &types.Activity{Duration: d}, next.nextState, next.successMessage
	}
	if currentState == askingActivityCount {
		val, err := strconv.ParseInt(command, 10, 64)
		if err != nil {
			return nil, askingActivityDuration, "Sorry, I can't understand that number."
		}
		return &types.Activity{Count: val}, next.nextState, next.successMessage
	}
	if currentState == askingActivityValue {
		val, errorMessage := parseActivityValue(command)
		if len(errorMessage) > 0 {
			return nil, askingActivityValue, errorMessage
		}
		return &types.Activity{Value: val}, next.nextState, next.successMessage
	}
	return nil, unknownStateType, "Sorry, the programmer messed this up. Please let them know."
}

// Returns the canonical value, or an error message
func parseActivityValue(command string) (string, string) {
	val, ok := map[string]string{
		"terrible":  "terrible",
		"awful":     "terrible",
		"bad":       "bad",
		"not good":  "bad",
		"neutral":   "neutral",
		"fine":      "neutral",
		"ok":        "neutral",
		"alright":   "neutral",
		"meh":       "neutral",
		"good":      "good",
		"great":     "great",
		"awesome":   "great",
		"fantastic": "great",
	}[command]
	if !ok {
		return "", "Sorry, I don't understand what that means, try saying something like \"ok\" or \"great\"!"
	}
	return val, ""
}

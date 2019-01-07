package conversation

import (
	"fmt"
	"log"
	"sort"
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
	statesToSkip map[stateType]struct{}

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
	witClient       WitClient
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

const (
	newUserWelcomeMessage = "Welcome! You can tell me activities that you've done and I'll remember them for you.\n" +
		"To get started, say something like \"day\", answer my questions, then say \"summary\" afterwards.\n" +
		"Please also tell me your timezone by saying \"timezone\" to make sure I know what day it is for you.\n" +
		"You can say \"help\" for other commands.\n"
	helpMessage = "Say \"activities\" to see the available activity types.\n" +
		"Say \"summary\" to see what you've recorded today.\n" +
		"Say \"timezone\" to see and change your timezone.\n" +
		"If you ever need to stop or quit recording a message, either word works."
)

func (m *managerImpl) Handle(fbID string, message string) string {
	message = strings.Replace(message, "\n", " ", -1)
	command := strings.ToLower(message)
	if command == "help" {
		return helpMessage
	}
	if command == "activities" {
		return "Available activities are the following: " +
			"day, programming, laundry, running, meetings, reading, yoga, climbing"
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	curState, ok := m.currentMessages[fbID]
	if !ok {
		// Load the user's information
		userID, timezone, newUser, err := m.database.AddOrGetUser(fbID, time.UTC)
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
		if newUser {
			return newUserWelcomeMessage
		}
		return "Welcome back! What activity do you want to record?"
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
		if command == "summary" {
			// TODO: Add an API for activity on a given day, then switch this to it.
			activities, err := m.database.ActivityForUser(curState.userID)
			if err != nil {
				return "There was an error fetching your summary. Try again shortly."
			}
			if len(activities) == 0 {
				return "You haven't recorded any activities yet today."
			}
			sort.Slice(activities, func(i, j int) bool {
				return activities[i].Type < activities[j].Type
			})
			summaries := make([]string, 0, len(activities))
			_, utcDate := nowAndUTCDate(time.Now(), curState.userTimezone)
			for _, activity := range activities {
				// Filter out ones not today.
				if activity.UTCDate.Unix() != utcDate.Unix() {
					continue
				}
				summaries = append(summaries, "-  "+summarizeActivity(activity))
			}
			return "Today you've recorded that:\n\n" + strings.Join(summaries, "\n")
		}

		// See if we can parse it the new fancy way.
		parsedWitMessage, errorMessage := parseMessage(m.witClient, message)
		if len(errorMessage) > 0 {
			return errorMessage
		}

		// Set up all the state properly to parse out the rest of the fields
		if parsedWitMessage.newActivity != nil {
			// Initialize the activity
			now, utcDate := nowAndUTCDate(time.Now(), curState.userTimezone)
			if parsedWitMessage.desiredTime != nil {
				now, utcDate = nowAndUTCDate(*parsedWitMessage.desiredTime, curState.userTimezone)
			}
			parsedWitMessage.newActivity.UTCDate = utcDate
			parsedWitMessage.newActivity.ActualTime = now
			parsedWitMessage.newActivity.RawMessages = message

			curState.activity = parsedWitMessage.newActivity
			curState.statesToSkip = parsedWitMessage.statesToSkip
			curState.currentActivityType = curState.activity.Type

			var startMessage string
			curState.currentState, startMessage = startForType(curState.activity.Type)
			curState.currentState, startMessage = fastForwardThroughSkippedStates(
				curState.statesToSkip,
				curState.currentActivityType,
				curState.currentState,
				startMessage,
			)
			return startMessage
		}

		// Otherwise, fall back to the simple parsing.
		activityType := determineActivityType(command)
		startState, startMessage := startForType(activityType)
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
			now, utcDate := nowAndUTCDate(time.Now(), curState.userTimezone)
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

		// Make sure we skip over any already-answered questions.
		nextState, response = fastForwardThroughSkippedStates(
			curState.statesToSkip,
			curState.currentActivityType,
			nextState,
			response,
		)

		// If this is the next state, we must be done!
		if nextState == askingActivityType {
			// Save the messages!
			// Note: This timezone value can always be changed later.
			userID, _, _, err := m.database.AddOrGetUser(fbID, time.UTC)
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
			response = "I finished writing that down, what activity type would you like to record next?"
		}
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

func nowAndUTCDate(now time.Time, userTimezone *time.Location) (time.Time, time.Time) {
	now = now.In(userTimezone)
	year, month, day := now.Date()
	utcDate, err := time.Parse("2006-01-02", fmt.Sprintf("%04d-%02d-%02d", year, month, day))
	if err != nil {
		panic(err)
	}
	return now, utcDate
}

func determineActivityType(command string) types.ActivityType {
	if command == "overall" || command == "overall day" || command == "day" {
		return types.ActivityOverallDay
	}
	if command == "programming" || command == "programmed" || command == "wrote code" || command == "coded" {
		return types.ActivityProgramming
	}
	if command == "laundry" {
		return types.ActivityLaundry
	}
	if command == "ran" || command == "went for a run" || command == "running" || command == "went running" {
		return types.ActivityRunning
	}
	if command == "met" || command == "meeting" || command == "meetings" {
		return types.ActivityMeetings
	}
	if command == "reading" || command == "read" {
		return types.ActivityReading
	}
	if command == "yoga" {
		return types.ActivityYoga
	}
	if command == "climbing" {
		return types.ActivityClimbing
	}
	return types.ActivityUnknown
}

var initialStates = map[types.ActivityType]successAndNextState{
	types.ActivityOverallDay: {
		successMessage: "How was your day?",
		nextState:      askingActivityValue,
	},
	types.ActivityProgramming: {
		successMessage: "How long did you program for?",
		nextState:      askingActivityDuration,
	},
	types.ActivityLaundry: {
		successMessage: "How many loads of laundry did you do?",
		nextState:      askingActivityCount,
	},
	types.ActivityRunning: {
		successMessage: "How far did you run in miles?",
		nextState:      askingActivityCount,
	},
	types.ActivityMeetings: {
		successMessage: "How many meetings did you go to?",
		nextState:      askingActivityCount,
	},
	types.ActivityReading: {
		successMessage: "How many pages did you read?",
		nextState:      askingActivityCount,
	},
	types.ActivityYoga: {
		successMessage: "How long did you do yoga for?",
		nextState:      askingActivityDuration,
	},
	types.ActivityClimbing: {
		successMessage: "How long did you climb for?",
		nextState:      askingActivityDuration,
	},
}

func startForType(activityType types.ActivityType) (stateType, string) {
	val := initialStates[activityType]
	return val.nextState, val.successMessage
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
	types.ActivityRunning: {
		askingActivityCount: successAndNextState{
			successMessage: "How long did you run for?",
			nextState:      askingActivityDuration,
		},
		askingActivityDuration: sentiment,
		askingActivityValue:    done,
	},
	types.ActivityMeetings: {
		askingActivityCount: successAndNextState{
			successMessage: "What was the total time you spent in meetings?",
			nextState:      askingActivityDuration,
		},
		askingActivityDuration: sentiment,
		askingActivityValue:    done,
	},
	types.ActivityReading: {
		askingActivityCount: successAndNextState{
			successMessage: "How long did you read for?",
			nextState:      askingActivityDuration,
		},
		askingActivityDuration: sentiment,
		askingActivityValue:    done,
	},
	types.ActivityYoga: {
		askingActivityDuration: sentiment,
		askingActivityValue:    done,
	},
	types.ActivityClimbing: {
		askingActivityDuration: sentiment,
		askingActivityValue:    done,
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

func fastForwardThroughSkippedStates(
	statesToSkip map[stateType]struct{},
	activityType types.ActivityType,
	curState stateType,
	curResponse string,
) (stateType, string) {
	if len(statesToSkip) == 0 {
		return curState, curResponse
	}
	for {
		if _, ok := statesToSkip[curState]; !ok {
			return curState, curResponse
		}
		// Otherwise, this is hard, we need to keep running through the proper flow.
		next, ok := handlerMap[activityType][curState]
		if !ok {
			return unknownStateType, "Sorry, the programmer messed this up. Please let them know."
		}
		curState = next.nextState
		curResponse = next.successMessage
	}
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

func sentimentEnding(value string) string {
	return fmt.Sprintf(" and felt %s about it.", value)
}

func shortDuration(duration time.Duration) string {
	s := duration.String()
	if strings.HasSuffix(s, "m0s") {
		s = s[:len(s)-2]
	}
	if strings.HasSuffix(s, "h0m") {
		s = s[:len(s)-2]
	}
	return s
}

func summarizeActivity(activity types.Activity) string {
	switch activity.Type {
	case types.ActivityOverallDay:
		return fmt.Sprintf("Your day was %s.", activity.Value)
	case types.ActivityProgramming:
		return fmt.Sprintf("You programmed for %s", shortDuration(activity.Duration)) + sentimentEnding(activity.Value)
	case types.ActivityLaundry:
		return fmt.Sprintf("You did %d loads of laundry", activity.Count) + sentimentEnding(activity.Value)
	case types.ActivityRunning:
		return fmt.Sprintf("You ran %d miles in %s", activity.Count, shortDuration(activity.Duration)) + sentimentEnding(activity.Value)
	case types.ActivityMeetings:
		return fmt.Sprintf("You spent %s in %d meetings", shortDuration(activity.Duration), activity.Count) + sentimentEnding(activity.Value)
	case types.ActivityReading:
		return fmt.Sprintf("You read %d pages in %s", activity.Count, shortDuration(activity.Duration)) + sentimentEnding(activity.Value)
	case types.ActivityYoga:
		return fmt.Sprintf("You did yoga for %s", shortDuration(activity.Duration)) + sentimentEnding(activity.Value)
	case types.ActivityClimbing:
		return fmt.Sprintf("You climbed for %s", shortDuration(activity.Duration)) + sentimentEnding(activity.Value)
	}
	return "Unknown activity."
}

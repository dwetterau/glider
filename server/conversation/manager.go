package conversation

import (
	"fmt"
	"log"
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
	activitiesToSave    []types.Activity
}

type stateType int

const (
	unknownStateType stateType = iota
	askingActivityType
	askingActivityValue
)

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
		if command != "start" {
			return "Sorry, your conversation might have timed out. Please start again"
		}
		// Start a new message!
		m.currentMessages[fbID] = &state{
			startTime:    time.Now(),
			lastMessage:  time.Now(),
			currentState: askingActivityType,
		}
		return "Hello! What type of activity do you want to record?"
	}
	// We already have a conversation going on, check for a few commands
	if command == "done" || command == "finished" {
		if curState.currentState != askingActivityType {
			return "Please finish recording your current activity first."
		}
		if len(curState.activitiesToSave) == 0 {
			delete(m.currentMessages, fbID)
			return "I don't have anything to record, but have a nice day!"
		}

		// TODO: Do this without holding the map lock...
		// Save the messages!
		userID, err := m.database.AddUser(fbID)
		if err != nil {
			log.Println("Error adding user: ", err.Error())
			return "Whoops, there was a problem saving your activity, try again shortly."
		}
		for _, activity := range curState.activitiesToSave {
			_, err = m.database.AddActivity(userID, activity)
			if err != nil {
				log.Println("Error saving activity: ", err.Error())
				return "Whoops, there was a problem saving your activity, try again shortly."
			}
		}
		return "Have a nice day!"
	}
	if command == "quit" || command == "abort" {
		delete(m.currentMessages, fbID)
		return "Have a nice day!"
	}

	// Finally, update the activity however the user wants to.
	if curState.currentState == askingActivityType {
		if command != "overall" && command != "overall day" && command != "day" {
			return "Sorry, I don't know what type of activity that is. " +
				"Try saying something like \"overall\"."
		}
		curState.currentActivityType = types.ActivityOverallDay
		curState.currentState = askingActivityValue
		return "How was your day?"
	} else if curState.currentState == askingActivityValue {
		if curState.currentActivityType == types.ActivityOverallDay {
			val, ok := map[string]string{
				"terrible":  "terrible",
				"awful":     "terrible",
				"bad":       "bad",
				"not good":  "bad",
				"neutral":   "neutral",
				"fine":      "neutral",
				"ok":        "neutral",
				"alright":   "neutral",
				"good":      "good",
				"great":     "great",
				"awesome":   "great",
				"fantastic": "great",
			}[command]
			if !ok {
				return "Sorry, I don't understand what that means, try saying something like \"ok\" or \"great\"!"
			}
			// TODO: This needs to respect the timezone of the actual user!
			now := time.Now()
			year, month, day := now.Date()
			utcDate, err := time.Parse("2006-01-02", fmt.Sprintf("%d-%d-%d", year, month, day))
			if err != nil {
				log.Println("Error parsing time: ", err.Error())
				return "Whoops, there was a problem figuring out the time."
			}

			curState.activitiesToSave = append(curState.activitiesToSave, types.Activity{
				Type:       curState.currentActivityType,
				UTCDate:    utcDate,
				ActualTime: now,
				Value:      val,
				RawValue:   message,
			})
			curState.currentState = askingActivityType
			return "Great! If you're finished, feel free to say so, " +
				"otherwise let me know what type of activity you want to record."
		}
		return "Sorry, the programmer messed this up. Please let them know."
	}
	return "Sorry, I can't understand what you're saying. You can say \"help\" for some help getting started."
}

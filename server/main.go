package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/dwetterau/glider/server/conversation"
	"github.com/dwetterau/glider/server/db"
	"github.com/dwetterau/glider/server/messenger"
	"github.com/wit-ai/wit-go"
)

const (
	fbVerifyTokenEnvName     = "FB_VERIFY_TOKEN"
	fbPageAccessTokenEnvName = "FB_PAGE_ACCESS_TOKEN"
	dbLocationEnvName        = "DB_LOCATION"
	witAITokenName           = "WIT_AI_TOKEN"
)

func main() {
	port := 8080

	flag.IntVar(&port, "port", port, "The port to listen on")
	flag.Parse()

	// Verify expected env variables
	for _, name := range []string{fbVerifyTokenEnvName, fbPageAccessTokenEnvName, dbLocationEnvName, witAITokenName} {
		if os.Getenv(name) == "" {
			log.Fatal("Missing env var: ", name)
		}
	}

	// Set up the db
	d, err := db.NewSQLite(os.Getenv(dbLocationEnvName))
	if err != nil {
		log.Fatal(err)
	}

	witClient := witai.NewClient(os.Getenv(witAITokenName))

	// Set up the conversation manager
	manager := conversation.New(d, witClient)

	http.HandleFunc("/", helloHandler)
	http.HandleFunc("/webhook", webhookHandler(manager))
	fmt.Println("Listening on", port)
	err = http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		log.Fatal(err)
	}
}

func helloHandler(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte("Nothing to see here."))
}

func webhookHandler(m conversation.Manager) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method == "GET" {
			challenge := req.URL.Query().Get("hub.challenge")
			token := req.URL.Query().Get("hub.verify_token")

			if token == os.Getenv(fbVerifyTokenEnvName) {
				w.WriteHeader(200)
				w.Write([]byte(challenge))
				return
			} else {
				w.WriteHeader(400)
				w.Write([]byte("wrong validation token"))
				return
			}
		} else if req.Method == "POST" {
			var callback messenger.Callback
			err := json.NewDecoder(req.Body).Decode(&callback)
			if err != nil {
				w.WriteHeader(500)
				w.Write([]byte(err.Error()))
				return
			}

			if callback.Object == "page" {
				for _, entry := range callback.Entry {
					for _, event := range entry.Messaging {
						err = process(m, event)
						if err != nil {
							log.Println("error processing message:", err, event)
						}
					}
				}
				w.WriteHeader(200)
				w.Write([]byte("processed all messages"))
				return
			} else {
				w.WriteHeader(400)
				w.Write([]byte("unsupported callback type"))
				return
			}
		}
		w.WriteHeader(400)
		w.Write([]byte("unsupported method"))
	}
}

func process(m conversation.Manager, event messenger.Messaging) error {
	client := &http.Client{}

	// Process the message and generate a reply
	reply := m.Handle(event.Sender.ID, event.Message.Text)

	response := messenger.Response{
		Recipient: messenger.User{
			ID: event.Sender.ID,
		},
		Message: messenger.Message{
			Text: reply,
		},
	}
	body := new(bytes.Buffer)
	err := json.NewEncoder(body).Encode(&response)
	if err != nil {
		return err
	}
	url := fmt.Sprintf(messenger.FacebookAPI, os.Getenv(fbPageAccessTokenEnvName))
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	// TODO: Read response?
	defer resp.Body.Close()
	return nil
}

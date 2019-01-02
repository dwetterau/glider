package main

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/dwetterau/glider/server/conversation"

	witai "github.com/wit-ai/wit-go"
)

func main() {
	witClient := witai.NewClient(os.Getenv("WIT_AI_TOKEN"))
	responses := make([]*witai.MessageResponse, len(conversation.TestMessages))
	for i, message := range conversation.TestMessages {
		var err error
		responses[i], err = witClient.Parse(&witai.MessageRequest{Query: message})
		if err != nil {
			panic(err)
		}
	}

	lines, err := json.Marshal(responses)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile("wit_output.json", lines, 0644)
	if err != nil {
		panic(err)
	}
}

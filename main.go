package main

import (
	"fmt"
	"glider/xscan"
	"time"
)

func main() {
	fmt.Println("Taking off!")
	scanner := xscan.New()
	for {
		name, err := scanner.CurrentWindowName()
		if err != nil {
			fmt.Printf("Encountered an error with xscan %v\n", err)
		} else {
			fmt.Printf("Currently focused window: %s\n", name)
		}
		time.Sleep(time.Duration(10 * time.Second))
	}
}

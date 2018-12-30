package local

import (
	"fmt"
	"time"

	"github.com/dwetterau/glider/local/annoy"
	"github.com/dwetterau/glider/local/xscan"
)

var sampleRate = 5 * time.Second

func main() {
	fmt.Println("Taking off!")
	scanner := xscan.New()
	annoyer := annoy.NewAnnoyer()
	lastWindow := xscan.Window{}
	for {
		window, err := scanner.CurrentWindow()
		if err != nil {
			fmt.Printf("Encountered an error with xscan %v\n", err)
		}
		duration := sampleRate
		if window != lastWindow {
			// If we switched into a new window, assume half the sample rate was spent on
			// the new window (on average).
			duration /= 2
		}
		annoyed := annoyer.MaybeAnnoy(window, sampleRate/2)
		if annoyed {
			fmt.Println("Annoyed for window: ", window)
			annoyer.Clear(window)
		}
		lastWindow = window
		time.Sleep(sampleRate)
	}
}

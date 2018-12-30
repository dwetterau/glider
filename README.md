Glider
=======
![Test Status](https://travis-ci.com/dwetterau/glider.svg?branch=master)

_Helping you soar_


## What does this do?
This binary watches what window you have focused and sends you notifications if you
are spending too much time in a given application. This might improve over time to be more
configurable, support more apps, etc.

Longer term, this tool might (or might not):
- Try to recommend smarter thresholds or tips to help improve your productivity
- Also watch your calendar and recommend cancelling meetings or blocked off time
- Integrate with an open task format and recommend tasks for you to work on when requested
- ???

## Requirements
- X11
- Go
- Only tested on Ubuntu (needs `notify-send` on path to send notifications)
- `xdotool`

## Setup

Fetch the code from this repo.

```
$ go get github.com/dwetterau/glider
```

Then run the main binary.

```
$ go run github.com/dwetterau/glider/main.go
```

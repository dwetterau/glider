Glider
=======
_Enabling programmers to soar_


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

## Setup

Install `xdotool`

```
apt install xdotool
```

Then run the main binary.

```
$ go run glider/main.go
```

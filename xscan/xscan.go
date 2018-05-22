package xscan

import (
	"errors"
	"fmt"
	"glider/tool"
	"regexp"
	"strings"
)

// Scans the currently open x windows and sees which ones have focus
type Scanner interface {
	CurrentWindow() (Window, error)
}

type Window struct {
	ApplicationName string
	Title           string
	PID             string
	WindowID        string
}

func New() Scanner {
	return scannerImpl{}
}

type scannerImpl struct{}

func (scannerImpl) CurrentWindow() (Window, error) {
	pidTitleWindow, err := tool.Run(
		"xdotool",
		"getwindowfocus", "getwindowpid", "getwindowname", "getwindowfocus")
	if err != nil {
		return Window{}, err
	}
	split := strings.Split(strings.TrimSpace(pidTitleWindow), "\n")
	if len(split) != 3 {
		return Window{}, errors.New(fmt.Sprintf("unexpected output from xdotool %s", pidTitleWindow))
	}
	pid := strings.TrimSpace(split[0])
	title := strings.TrimSpace(split[1])
	windowID := strings.TrimSpace(split[2])

	xprops, err := tool.Run("xprop", "-id", windowID)
	if err != nil {
		return Window{}, err
	}

	return Window{
		ApplicationName: applicationNameFromXProps(xprops),
		Title:           title,
		PID:             pid,
		WindowID:        windowID,
	}, nil
}

var nameRegex = regexp.MustCompile(`WM_CLASS.*"(?P<Name>\S*)"$`)

func applicationNameFromXProps(xprops string) string {
	for _, line := range strings.Split(xprops, "\n") {
		matches := nameRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		// We found the right line, extract the value out!
		return matches[1]
	}
	return ""
}

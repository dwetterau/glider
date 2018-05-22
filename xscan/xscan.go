package xscan

import (
	"os/exec"
	"bytes"
	"strings"
	"errors"
	"fmt"
	"io"
)

// Scans the currently open x windows and sees which ones have focus
type Scanner interface {
	CurrentWindowName() (string, error)
}

func New() Scanner {
	return scannerImpl{}
}

type scannerImpl struct {}

func (scannerImpl) CurrentWindowName() (string, error) {
	pidAndTitle, err := runCommand("xdotool", "getwindowfocus", "getwindowpid", "getwindowname")
	if err != nil {
		return "", err
	}
	splitPidAndTitle := strings.Split(pidAndTitle, "/")
	if len(splitPidAndTitle) != 2 {
		return "", errors.New(fmt.Sprintf("unexpected output from xdotool %s", pidAndTitle))
	}
	pid := splitPidAndTitle[0]
	title := splitPidAndTitle[1]

	processName, err := runCommand("ps", "-p", pid, "-o", "comm=")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Process: %s, Title: %s", processName, title), nil
}

func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	stdout, err := cmd.CombinedOutput()
	return string(stdout), err
}

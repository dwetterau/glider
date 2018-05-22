package annoy

import (
	"glider/tool"
	"glider/xscan"
	"regexp"
	"time"
)

type annoyingApplication int

const (
	Unknown annoyingApplication = iota
	Slack
	GMail
)

var thresholds = map[annoyingApplication]time.Duration{
	Slack: 5 * time.Minute,
	GMail: 5 * time.Minute,
}

type Annoyer interface {
	MaybeAnnoy(window xscan.Window, duration time.Duration) bool
	Clear(window xscan.Window)
}

func NewAnnoyer() Annoyer {
	return &annoyerImpl{
		durations: map[annoyingApplication]time.Duration{
			Unknown: 0,
			Slack:   0,
			GMail:   0,
		},
	}
}

type annoyerImpl struct {
	durations map[annoyingApplication]time.Duration
}

func (a *annoyerImpl) MaybeAnnoy(window xscan.Window, duration time.Duration) bool {
	application := classify(window)
	if application == Unknown {
		return false
	}

	a.durations[application] += duration
	if a.durations[application] > thresholds[application] {
		tool.Run("notify-send", "Glider", message(application))
		return true
	}
}

func (a *annoyerImpl) Clear(window xscan.Window) {
	a.durations[classify(window)] = 0
}

var (
	slackRegex = regexp.MustCompile(` \| .* Slack\b`)
	gmailRegex = regexp.MustCompile(`\S+@\S+\.\S+.+G?[mM]ail -`)
)

func classify(window xscan.Window) annoyingApplication {
	foundSlack := slackRegex.Find([]byte(window.Title))
	if foundSlack != nil {
		return Slack
	}
	foundGmail := gmailRegex.Find([]byte(window.Title))
	if foundGmail != nil {
		return GMail
	}
	return Unknown
}

func message(application annoyingApplication) string {
	switch application {
	case Slack:
		return "Stop reading Slack."
	case GMail:
		return "Read your email faster or not at all."
	default:
		return "Shouldn't you be doing something else?"
	}
}

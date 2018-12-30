package annoy

import (
	"regexp"
	"time"

	"glider/tool"
	"glider/xscan"
)

type annoyingApplication int

const (
	Unknown annoyingApplication = iota
	Slack
	GMail
)

var thresholds = map[annoyingApplication]threshold{
	Slack: {3 * time.Minute, .5},
	GMail: {5 * time.Minute, 10},
}

type threshold struct {
	// The annoyer will annoy if an application accumulates this amount of duration.
	bucketSize time.Duration

	// A value in [0, inf) that is multiplied by the time elapsed and subtracted from the
	// accumulated bucket when the application is not active.
	drainFactor float64
}

type Annoyer interface {
	MaybeAnnoy(window xscan.Window, duration time.Duration) bool
	Clear(window xscan.Window)
}

func NewAnnoyer() Annoyer {
	return &annoyerImpl{
		buckets: map[annoyingApplication]time.Duration{
			Unknown: 0,
			Slack:   0,
			GMail:   0,
		},
	}
}

type annoyerImpl struct {
	buckets map[annoyingApplication]time.Duration
}

func (a *annoyerImpl) MaybeAnnoy(window xscan.Window, duration time.Duration) bool {
	application := classify(window)
	a.drainBuckets(application, duration)
	if application == Unknown {
		return false
	}

	a.buckets[application] += duration
	if a.buckets[application] > thresholds[application].bucketSize {
		tool.Run("notify-send", "Glider", message(application))
		return true
	}
	return false
}

func (a *annoyerImpl) drainBuckets(curApplication annoyingApplication, duration time.Duration) {
	for application, bucket := range a.buckets {
		if application == curApplication {
			continue
		}
		a.buckets[application] = bucket - time.Duration(thresholds[application].drainFactor*float64(duration))
		if a.buckets[application] < 0 {
			a.buckets[application] = 0
		}
	}
}

func (a *annoyerImpl) Clear(window xscan.Window) {
	a.buckets[classify(window)] = 0
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

package scheduler

import (
	"testing"
	"time"

	"proxy-hub/internal/config"
	"proxy-hub/internal/router"
)

func TestPickPriorityAndRoundRobin(t *testing.T) {
	s := New(Options{})
	hits := []router.Hit{
		hit("b", 20),
		hit("a", 10),
		hit("c", 10),
	}
	first := s.Pick(hits, 1)
	second := s.Pick(hits, 1)
	if first[0].Hit.ChannelName != "a" {
		t.Fatalf("first = %s, want a", first[0].Hit.ChannelName)
	}
	if second[0].Hit.ChannelName != "c" {
		t.Fatalf("second = %s, want c", second[0].Hit.ChannelName)
	}
}

func TestPickRotatesAPIKeys(t *testing.T) {
	s := New(Options{})
	h := hit("a", 10)
	h.APIKeyEntries = []config.APIKeyEntry{{APIKey: "k1"}, {APIKey: "k2"}}
	first := s.Pick([]router.Hit{h}, 1)
	second := s.Pick([]router.Hit{h}, 1)
	if first[0].APIKey != "k1" || first[0].APIKeyEntryIndex != 0 {
		t.Fatalf("first key = %+v", first[0])
	}
	if second[0].APIKey != "k2" || second[0].APIKeyEntryIndex != 1 {
		t.Fatalf("second key = %+v", second[0])
	}
}

func TestCircuitSkipsOpenChannel(t *testing.T) {
	s := New(Options{Cooldown: time.Hour, FailureThreshold: 1})
	s.ReportFailure("a")
	got := s.Pick([]router.Hit{hit("a", 10), hit("b", 20)}, 2)
	if len(got) != 1 || got[0].Hit.ChannelName != "b" {
		t.Fatalf("Pick() = %+v, want only b", got)
	}
}

func hit(name string, priority int) router.Hit {
	return router.Hit{
		ChannelName:   name,
		ChannelType:   config.ChannelTypeOpenAIAPI,
		Priority:      priority,
		APIKeyEntries: []config.APIKeyEntry{{APIKey: "sk-" + name}},
	}
}

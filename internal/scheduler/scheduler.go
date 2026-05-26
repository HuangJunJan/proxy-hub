package scheduler

import (
	"sort"
	"strconv"
	"sync"
	"time"

	"proxy-hub/internal/config"
	"proxy-hub/internal/router"
)

type Scheduler struct {
	mu               sync.Mutex
	bucketCursors    map[string]uint64
	keyCursors       map[string]uint64
	circuits         map[string]*circuitState
	cooldown         time.Duration
	failureThreshold int
}

type Options struct {
	Cooldown         time.Duration
	FailureThreshold int
}

type Selection struct {
	Hit              router.Hit
	APIKey           string
	APIKeyEntryIndex int
}

func New(opts Options) *Scheduler {
	if opts.Cooldown <= 0 {
		opts.Cooldown = time.Duration(config.DefaultCircuitCooldownSec) * time.Second
	}
	if opts.FailureThreshold <= 0 {
		opts.FailureThreshold = config.DefaultCircuitFailureThreshold
	}
	return &Scheduler{
		bucketCursors:    map[string]uint64{},
		keyCursors:       map[string]uint64{},
		circuits:         map[string]*circuitState{},
		cooldown:         opts.Cooldown,
		failureThreshold: opts.FailureThreshold,
	}
}

func (s *Scheduler) Pick(hits []router.Hit, maxAttempts int) []Selection {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	alive := make([]router.Hit, 0, len(hits))
	for _, hit := range hits {
		if s.isAvailableLocked(hit.ChannelName) {
			alive = append(alive, hit)
		}
	}
	sort.SliceStable(alive, func(i, j int) bool {
		if alive[i].Priority == alive[j].Priority {
			if alive[i].ChannelType == alive[j].ChannelType {
				return alive[i].ChannelName < alive[j].ChannelName
			}
			return alive[i].ChannelType < alive[j].ChannelType
		}
		return alive[i].Priority < alive[j].Priority
	})

	var selections []Selection
	for _, bucket := range priorityBuckets(alive) {
		rotated := s.rotateLocked(bucket)
		for _, hit := range rotated {
			selection := Selection{Hit: hit, APIKeyEntryIndex: -1}
			if hit.ChannelType == config.ChannelTypeOpenAIAPI {
				apiKey, idx, ok := s.nextAPIKeyLocked(hit.ChannelName, hit.APIKeyEntries)
				if !ok {
					continue
				}
				selection.APIKey = apiKey
				selection.APIKeyEntryIndex = idx
			}
			selections = append(selections, selection)
			if len(selections) >= maxAttempts {
				return selections
			}
		}
	}
	return selections
}

func (s *Scheduler) ReportSuccess(channelName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.stateLocked(channelName)
	state.failures = 0
	state.openedAt = time.Time{}
	state.open = false
}

func (s *Scheduler) ReportFailure(channelName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.stateLocked(channelName)
	state.failures++
	if state.failures >= s.failureThreshold {
		state.open = true
		state.openedAt = time.Now()
	}
}

func (s *Scheduler) IsOpen(channelName string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.stateLocked(channelName)
	return state.open && time.Since(state.openedAt) < s.cooldown
}

func (s *Scheduler) isAvailableLocked(channelName string) bool {
	state := s.stateLocked(channelName)
	if !state.open {
		return true
	}
	if time.Since(state.openedAt) >= s.cooldown {
		return true
	}
	return false
}

func (s *Scheduler) stateLocked(channelName string) *circuitState {
	state := s.circuits[channelName]
	if state == nil {
		state = &circuitState{}
		s.circuits[channelName] = state
	}
	return state
}

func (s *Scheduler) rotateLocked(bucket []router.Hit) []router.Hit {
	if len(bucket) <= 1 {
		return append([]router.Hit(nil), bucket...)
	}
	key := bucketKey(bucket)
	cursor := s.bucketCursors[key]
	s.bucketCursors[key] = cursor + 1
	start := int(cursor % uint64(len(bucket)))
	out := make([]router.Hit, 0, len(bucket))
	out = append(out, bucket[start:]...)
	out = append(out, bucket[:start]...)
	return out
}

func (s *Scheduler) nextAPIKeyLocked(channelName string, entries []config.APIKeyEntry) (string, int, bool) {
	if len(entries) == 0 {
		return "", -1, false
	}
	cursor := s.keyCursors[channelName]
	s.keyCursors[channelName] = cursor + 1
	idx := int(cursor % uint64(len(entries)))
	if entries[idx].APIKey == "" {
		return "", -1, false
	}
	return entries[idx].APIKey, idx, true
}

type circuitState struct {
	failures int
	open     bool
	openedAt time.Time
}

func priorityBuckets(hits []router.Hit) [][]router.Hit {
	if len(hits) == 0 {
		return nil
	}
	var buckets [][]router.Hit
	start := 0
	for i := 1; i <= len(hits); i++ {
		if i == len(hits) || hits[i].Priority != hits[start].Priority {
			buckets = append(buckets, hits[start:i])
			start = i
		}
	}
	return buckets
}

func bucketKey(bucket []router.Hit) string {
	if len(bucket) == 0 {
		return ""
	}
	return bucket[0].ChannelType + ":" + strconv.Itoa(bucket[0].Priority)
}

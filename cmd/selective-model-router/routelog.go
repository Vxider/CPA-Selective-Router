package main

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// routeLogCapacity bounds the in-memory ring buffer used for the log table.
const routeLogCapacity = 500

// routeBucketSize is the duration each progress-strip cell represents.
const routeBucketSize = 10 * time.Minute

// routeBucketWindow is the total time span the progress strip always shows.
const routeBucketWindow = 3 * time.Hour

// routeLogEvent records one routing decision in memory.
type routeLogEvent struct {
	Time           time.Time `json:"time"`
	Phase          string    `json:"phase"`
	RequestedModel string    `json:"requested_model,omitempty"`
	SourceFormat   string    `json:"source_format,omitempty"`
	Stream         bool      `json:"stream,omitempty"`
	Handled        bool      `json:"handled"`
	Category       string    `json:"category,omitempty"`
	Reason         string    `json:"reason,omitempty"`
	TargetProvider string    `json:"target_provider,omitempty"`
	TargetModel    string    `json:"target_model,omitempty"`
}

// routeLogStats aggregates recorded events for the dashboard.
type routeLogStats struct {
	GeneratedAt       time.Time       `json:"generated_at"`
	FirstSeen         time.Time       `json:"first_seen,omitempty"`
	LastSeen          time.Time       `json:"last_seen,omitempty"`
	TotalRoutes       int             `json:"total_routes"`
	HandledRoutes     int             `json:"handled_routes"`
	ByCategory        map[string]int  `json:"by_category"`
	ByReason          map[string]int  `json:"by_reason"`
	ByRequestedModel  map[string]int  `json:"by_requested_model"`
	ByTargetProvider  map[string]int  `json:"by_target_provider"`
	RecentCapacity    int             `json:"recent_capacity"`
	RecentCount       int             `json:"recent_count"`
	CategoriesOrdered []categoryCount `json:"categories_ordered"`
	Buckets           []routeBucket   `json:"buckets"`
}

type categoryCount struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
	Handled  bool   `json:"handled"`
}

// routeBucket is one time-windowed cell in the progress strip.
type routeBucket struct {
	Start      time.Time      `json:"start"`
	End        time.Time      `json:"end"`
	Total      int            `json:"total"`
	ByCategory map[string]int `json:"by_category"`
}

// bucketCell holds cumulative counts for a single 10-minute window.
type bucketCell struct {
	total int
	byCat map[string]int
}

// routeLogStore is an in-memory record of routing decisions. It maintains:
//   - cumulative counters (never evicted) for accurate totals matching system logs
//   - time-bucketed counters for the 3-hour progress strip
//   - a ring buffer for the recent log table display
type routeLogStore struct {
	mu sync.Mutex

	// Ring buffer for log table (bounded).
	events  []routeLogEvent
	cursor  int
	wrapped bool

	// Cumulative counters (never evicted by ring buffer wrapping).
	totalRoutes   int
	handledRoutes int
	byCategory    map[string]int
	byReason      map[string]int
	byModel       map[string]int
	byProvider    map[string]int
	firstSeen     time.Time
	lastSeen      time.Time

	// Time-bucketed counters for progress strip.
	bucketCells map[time.Time]*bucketCell
}

var routeLog = newRouteLogStore()

func newRouteLogStore() *routeLogStore {
	return &routeLogStore{
		byCategory:  map[string]int{},
		byReason:    map[string]int{},
		byModel:     map[string]int{},
		byProvider:  map[string]int{},
		bucketCells: map[time.Time]*bucketCell{},
	}
}

func (s *routeLogStore) record(ev routeLogEvent) {
	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}
	cat := ev.Category
	if cat == "" {
		cat = "normal"
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// --- Ring buffer (for log table) ---
	if s.events == nil {
		s.events = make([]routeLogEvent, routeLogCapacity)
	}
	s.events[s.cursor] = ev
	s.cursor = (s.cursor + 1) % routeLogCapacity
	if s.cursor == 0 {
		s.wrapped = true
	}

	// --- Cumulative counters ---
	s.totalRoutes++
	if ev.Handled {
		s.handledRoutes++
	}
	s.byCategory[cat]++
	if ev.Reason != "" {
		s.byReason[ev.Reason]++
	}
	if ev.RequestedModel != "" {
		s.byModel[ev.RequestedModel]++
	}
	if ev.TargetProvider != "" {
		s.byProvider[ev.TargetProvider]++
	}
	if s.firstSeen.IsZero() || ev.Time.Before(s.firstSeen) {
		s.firstSeen = ev.Time
	}
	if ev.Time.After(s.lastSeen) {
		s.lastSeen = ev.Time
	}

	// --- Time bucket for progress strip ---
	bt := ev.Time.Truncate(routeBucketSize)
	cell, ok := s.bucketCells[bt]
	if !ok {
		cell = &bucketCell{byCat: map[string]int{}}
		s.bucketCells[bt] = cell
	}
	cell.total++
	cell.byCat[cat]++
}

// snapshot returns the ring buffer contents newest-first for log display.
func (s *routeLogStore) snapshot() []routeLogEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ordered []routeLogEvent
	if s.wrapped {
		ordered = make([]routeLogEvent, 0, routeLogCapacity)
		ordered = append(ordered, s.events[s.cursor:]...)
		ordered = append(ordered, s.events[:s.cursor]...)
	} else {
		ordered = append(ordered, s.events[:s.cursor]...)
	}
	out := make([]routeLogEvent, len(ordered))
	for i, ev := range ordered {
		out[len(ordered)-1-i] = ev
	}
	return out
}

func (s *routeLogStore) stats() routeLogStats {
	s.mu.Lock()
	stats := routeLogStats{
		GeneratedAt:      time.Now(),
		TotalRoutes:      s.totalRoutes,
		HandledRoutes:    s.handledRoutes,
		ByCategory:       copyMap(s.byCategory),
		ByReason:         copyMap(s.byReason),
		ByRequestedModel: copyMap(s.byModel),
		ByTargetProvider: copyMap(s.byProvider),
		FirstSeen:        s.firstSeen,
		LastSeen:         s.lastSeen,
		RecentCapacity:   routeLogCapacity,
		Buckets:          s.buildBucketsLocked(),
	}
	recentCount := s.cursor
	if s.wrapped {
		recentCount = routeLogCapacity
	}
	stats.RecentCount = recentCount
	s.mu.Unlock()

	stats.CategoriesOrdered = orderedCategories(stats.ByCategory)
	return stats
}

// buildBucketsLocked assembles the fixed 3-hour progress strip from bucketCells.
func (s *routeLogStore) buildBucketsLocked() []routeBucket {
	const maxBuckets = int(routeBucketWindow / routeBucketSize) // 18
	now := time.Now()
	end := now.Truncate(routeBucketSize).Add(routeBucketSize)
	start := end.Add(-routeBucketWindow)

	out := make([]routeBucket, 0, maxBuckets)
	cur := start
	for i := 0; i < maxBuckets; i++ {
		rb := routeBucket{Start: cur, End: cur.Add(routeBucketSize), ByCategory: map[string]int{}}
		if cell := s.bucketCells[cur]; cell != nil {
			rb.Total = cell.total
			for k, v := range cell.byCat {
				rb.ByCategory[k] = v
			}
		}
		out = append(out, rb)
		cur = cur.Add(routeBucketSize)
	}

	// Prune buckets older than the window to avoid unbounded growth.
	windowStart := start
	for t := range s.bucketCells {
		if t.Before(windowStart) {
			delete(s.bucketCells, t)
		}
	}

	return out
}

func copyMap(m map[string]int) map[string]int {
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func orderedCategories(counts map[string]int) []categoryCount {
	handled := map[string]bool{
		"normal":                     false,
		"compact":                    true,
		"auto_review":                true,
		"web_search":                 true,
		"vision":                     true,
		"image_generation":           true,
		"disabled":                   false,
		"route_provider_unavailable": false,
	}
	out := make([]categoryCount, 0, len(counts))
	for name, count := range counts {
		out = append(out, categoryCount{Category: name, Count: count, Handled: handled[strings.TrimSpace(name)]})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Category < out[j].Category
	})
	return out
}

func (s *routeLogStore) clear() {
	s.mu.Lock()
	s.events = nil
	s.cursor = 0
	s.wrapped = false
	s.totalRoutes = 0
	s.handledRoutes = 0
	s.byCategory = map[string]int{}
	s.byReason = map[string]int{}
	s.byModel = map[string]int{}
	s.byProvider = map[string]int{}
	s.firstSeen = time.Time{}
	s.lastSeen = time.Time{}
	s.bucketCells = map[time.Time]*bucketCell{}
	s.mu.Unlock()
}

// recordRouteDecision records a model.route decision.
func recordRouteDecision(req rpcModelRouteRequest, cfg pluginConfig, decision routeDecision, reason string, handled bool) {
	category := "normal"
	if reason != "no_match" {
		category = reason
	}
	ev := routeLogEvent{
		Phase:          "route",
		RequestedModel: strings.TrimSpace(req.RequestedModel),
		SourceFormat:   strings.TrimSpace(req.SourceFormat),
		Stream:         req.Stream,
		Handled:        handled,
		Reason:         reason,
		Category:       category,
	}
	if decision.Handled {
		ev.Handled = true
		ev.Category = decision.Category
		ev.TargetProvider = strings.TrimSpace(decision.Provider)
		ev.TargetModel = strings.TrimSpace(decision.Model)
	}
	_ = cfg
	routeLog.record(ev)
}

package observability

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ProcessMetrics stores low-cardinality process counters and duration summaries.
// Callers must use fixed metric names and labels that never contain user data.
type ProcessMetrics struct {
	mu        sync.Mutex
	counters  map[string]float64
	gauges    map[string]float64
	durations map[string]durationSummary
	refreshes map[string]time.Time
}

type durationSummary struct {
	count uint64
	sum   float64
}

var DefaultMetrics = NewProcessMetrics()

func NewProcessMetrics() *ProcessMetrics {
	return &ProcessMetrics{counters: make(map[string]float64), gauges: make(map[string]float64), durations: make(map[string]durationSummary), refreshes: make(map[string]time.Time)}
}

func Bucket100(value int) string {
	switch {
	case value >= 90:
		return "90_100"
	case value >= 80:
		return "80_89"
	case value >= 70:
		return "70_79"
	default:
		return "0_69"
	}
}

func (m *ProcessMetrics) SetGauge(name string, labels map[string]string, value float64) {
	if m == nil {
		return
	}
	key := seriesKey(name, labels)
	m.mu.Lock()
	m.gauges[key] = value
	m.mu.Unlock()
}

// ShouldRefresh limits periodic queue-stat queries across worker iterations.
func (m *ProcessMetrics) ShouldRefresh(key string, now time.Time, interval time.Duration) bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if last, ok := m.refreshes[key]; ok && now.Sub(last) < interval {
		return false
	}
	m.refreshes[key] = now
	return true
}

func (m *ProcessMetrics) Add(name string, labels map[string]string, value float64) {
	if m == nil || value == 0 {
		return
	}
	key := seriesKey(name, labels)
	m.mu.Lock()
	m.counters[key] += value
	m.mu.Unlock()
}

func (m *ProcessMetrics) ObserveDuration(name string, labels map[string]string, value time.Duration) {
	if m == nil || value < 0 {
		return
	}
	key := seriesKey(name, labels)
	m.mu.Lock()
	summary := m.durations[key]
	summary.count++
	summary.sum += value.Seconds()
	m.durations[key] = summary
	m.mu.Unlock()
}

// PrometheusText renders counters and duration summaries in the text exposition format.
func (m *ProcessMetrics) PrometheusText() string {
	if m == nil {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	var out strings.Builder
	counters := make([]string, 0, len(m.counters))
	for key := range m.counters {
		counters = append(counters, key)
	}
	sort.Strings(counters)
	seen := make(map[string]struct{})
	for _, key := range counters {
		name, labels := parseSeriesKey(key)
		if _, ok := seen[name]; !ok {
			fmt.Fprintf(&out, "# TYPE %s counter\n", name)
			seen[name] = struct{}{}
		}
		fmt.Fprintf(&out, "%s%s %g\n", name, renderLabels(labels), m.counters[key])
	}
	gauges := make([]string, 0, len(m.gauges))
	for key := range m.gauges {
		gauges = append(gauges, key)
	}
	sort.Strings(gauges)
	seen = make(map[string]struct{})
	for _, key := range gauges {
		name, labels := parseSeriesKey(key)
		if _, ok := seen[name]; !ok {
			fmt.Fprintf(&out, "# TYPE %s gauge\n", name)
			seen[name] = struct{}{}
		}
		fmt.Fprintf(&out, "%s%s %g\n", name, renderLabels(labels), m.gauges[key])
	}
	durations := make([]string, 0, len(m.durations))
	for key := range m.durations {
		durations = append(durations, key)
	}
	sort.Strings(durations)
	seen = make(map[string]struct{})
	for _, key := range durations {
		name, labels := parseSeriesKey(key)
		summary := m.durations[key]
		if _, ok := seen[name]; !ok {
			fmt.Fprintf(&out, "# TYPE %s summary\n", name)
			seen[name] = struct{}{}
		}
		fmt.Fprintf(&out, "%s_seconds_sum%s %g\n", name, renderLabels(labels), summary.sum)
		fmt.Fprintf(&out, "%s_seconds_count%s %d\n", name, renderLabels(labels), summary.count)
	}
	return out.String()
}

func seriesKey(name string, labels map[string]string) string {
	encoded, _ := json.Marshal(labels)
	return name + "|" + string(encoded)
}

func parseSeriesKey(key string) (string, map[string]string) {
	name, encoded, _ := strings.Cut(key, "|")
	labels := map[string]string{}
	_ = json.Unmarshal([]byte(encoded), &labels)
	return name, labels
}

func renderLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		value := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(labels[key])
		values = append(values, fmt.Sprintf(`%s="%s"`, key, value))
	}
	return "{" + strings.Join(values, ",") + "}"
}

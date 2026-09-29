package observability

import (
	"strings"
	"testing"
	"time"
)

func TestProcessMetricsExposeCountersAndDurationsWithoutBreakingLabels(t *testing.T) {
	metrics := NewProcessMetrics()
	metrics.Add("hireradar_jobs_fetched_total", map[string]string{"provider": "ats|x=\"a\""}, 3)
	metrics.ObserveDuration("hireradar_matching_duration", map[string]string{"event_type": "job.created"}, 1500*time.Millisecond)
	metrics.SetGauge("hireradar_matching_queue_depth", nil, 4)

	output := metrics.PrometheusText()
	for _, expected := range []string{
		"# TYPE hireradar_jobs_fetched_total counter",
		`hireradar_jobs_fetched_total{provider="ats|x=\"a\""} 3`,
		"# TYPE hireradar_matching_duration summary",
		`hireradar_matching_duration_seconds_sum{event_type="job.created"} 1.5`,
		`hireradar_matching_duration_seconds_count{event_type="job.created"} 1`,
		"# TYPE hireradar_matching_queue_depth gauge",
		"hireradar_matching_queue_depth 4",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("metrics output missing %q:\n%s", expected, output)
		}
	}
}

func TestShouldRefreshThrottlesQueueQueries(t *testing.T) {
	metrics := NewProcessMetrics()
	now := time.Now()
	if !metrics.ShouldRefresh("queue", now, time.Minute) {
		t.Fatal("first refresh should run")
	}
	if metrics.ShouldRefresh("queue", now.Add(30*time.Second), time.Minute) {
		t.Fatal("refresh ran before interval elapsed")
	}
	if !metrics.ShouldRefresh("queue", now.Add(time.Minute), time.Minute) {
		t.Fatal("refresh should run after interval elapsed")
	}
}

func TestBucket100(t *testing.T) {
	for value, expected := range map[int]string{0: "0_69", 69: "0_69", 70: "70_79", 79: "70_79", 80: "80_89", 89: "80_89", 90: "90_100", 100: "90_100"} {
		if actual := Bucket100(value); actual != expected {
			t.Fatalf("Bucket100(%d)=%q, want %q", value, actual, expected)
		}
	}
}

package metrics

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockExporter implements CloudExporter for testing.
type mockExporter struct {
	name    string
	cloud   string
	metrics []Metric
	err     error
	started bool
	stopped bool
}

func (m *mockExporter) Name() string  { return m.name }
func (m *mockExporter) Cloud() string { return m.cloud }
func (m *mockExporter) Start(_ context.Context) error {
	m.started = true
	return nil
}
func (m *mockExporter) Stop() error {
	m.stopped = true
	return nil
}
func (m *mockExporter) CollectMetrics(_ context.Context) ([]Metric, error) {
	return m.metrics, m.err
}

// failStartExporter returns an error from Start.
type failStartExporter struct{ mockExporter }

func (f *failStartExporter) Start(_ context.Context) error {
	return errors.New("start failed")
}

// ---- Metric struct tests ----

func TestMetric_ZeroValue(t *testing.T) {
	var m Metric
	if m.Cloud != "" {
		t.Errorf("expected empty Cloud, got %q", m.Cloud)
	}
	if !m.Timestamp.IsZero() {
		t.Error("expected zero timestamp")
	}
}

func TestMetric_Fields(t *testing.T) {
	ts := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	m := Metric{
		Name:      "CPUUtilization",
		Cloud:     "aws",
		Namespace: "AWS/EC2",
		Value:     75.5,
		Timestamp: ts,
		Unit:      "Percent",
		Statistic: "Average",
		Labels:    map[string]string{"region": "us-east-1"},
		Dimensions: map[string]string{"InstanceId": "i-abc123"},
	}

	if m.Name != "CPUUtilization" {
		t.Errorf("unexpected Name: %q", m.Name)
	}
	if m.Value != 75.5 {
		t.Errorf("unexpected Value: %f", m.Value)
	}
	if m.Labels["region"] != "us-east-1" {
		t.Errorf("unexpected label region: %q", m.Labels["region"])
	}
}

// ---- Aggregator tests ----

func TestNewAggregator_DefaultInterval(t *testing.T) {
	agg := NewAggregator(AggregatorConfig{})
	if agg.config.CollectInterval != 60*time.Second {
		t.Errorf("expected 60s default interval, got %v", agg.config.CollectInterval)
	}
}

func TestNewAggregator_CustomInterval(t *testing.T) {
	agg := NewAggregator(AggregatorConfig{CollectInterval: 30 * time.Second})
	if agg.config.CollectInterval != 30*time.Second {
		t.Errorf("expected 30s interval, got %v", agg.config.CollectInterval)
	}
}

func TestAggregator_Start_StartsAllExporters(t *testing.T) {
	exp1 := &mockExporter{name: "exp1", cloud: "aws"}
	exp2 := &mockExporter{name: "exp2", cloud: "gcp"}

	agg := NewAggregator(AggregatorConfig{
		CollectInterval: 10 * time.Second,
		Exporters:       []CloudExporter{exp1, exp2},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := agg.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	if !exp1.started {
		t.Error("exp1 was not started")
	}
	if !exp2.started {
		t.Error("exp2 was not started")
	}
}

func TestAggregator_Start_PropagatesExporterError(t *testing.T) {
	good := &mockExporter{name: "good", cloud: "aws"}
	bad := &failStartExporter{mockExporter{name: "bad", cloud: "gcp"}}

	agg := NewAggregator(AggregatorConfig{
		CollectInterval: 10 * time.Second,
		Exporters:       []CloudExporter{good, bad},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := agg.Start(ctx)
	if err == nil {
		t.Error("expected error when exporter Start fails, got nil")
	}
}

func TestAggregator_Shutdown_StopsAllExporters(t *testing.T) {
	exp1 := &mockExporter{name: "exp1", cloud: "aws"}
	exp2 := &mockExporter{name: "exp2", cloud: "azure"}

	agg := NewAggregator(AggregatorConfig{
		CollectInterval: 10 * time.Second,
		Exporters:       []CloudExporter{exp1, exp2},
	})

	ctx, cancel := context.WithCancel(context.Background())
	if err := agg.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	cancel()

	if err := agg.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown returned error: %v", err)
	}

	if !exp1.stopped {
		t.Error("exp1 was not stopped")
	}
	if !exp2.stopped {
		t.Error("exp2 was not stopped")
	}
}

func TestAggregator_Collect_FanOutToExporters(t *testing.T) {
	ts := time.Now()
	exp1 := &mockExporter{
		name:  "aws",
		cloud: "aws",
		metrics: []Metric{
			{Name: "cpu", Cloud: "aws", Value: 50.0, Timestamp: ts},
		},
	}
	exp2 := &mockExporter{
		name:  "gcp",
		cloud: "gcp",
		metrics: []Metric{
			{Name: "cpu", Cloud: "gcp", Value: 30.0, Timestamp: ts},
			{Name: "memory", Cloud: "gcp", Value: 60.0, Timestamp: ts},
		},
	}

	agg := NewAggregator(AggregatorConfig{
		CollectInterval: 10 * time.Second,
		Exporters:       []CloudExporter{exp1, exp2},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := agg.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Manually trigger collect to avoid waiting for ticker.
	agg.collect(ctx)

	// Drain the channel and count metrics.
	got := 0
	for i := 0; i < 3; i++ {
		select {
		case <-agg.metrics:
			got++
		case <-time.After(100 * time.Millisecond):
		}
	}

	if got != 3 {
		t.Errorf("expected 3 metrics in buffer, got %d", got)
	}
}

func TestAggregator_Collect_SkipsExporterOnError(t *testing.T) {
	ts := time.Now()
	good := &mockExporter{
		name:    "good",
		cloud:   "aws",
		metrics: []Metric{{Name: "cpu", Cloud: "aws", Value: 42.0, Timestamp: ts}},
	}
	bad := &mockExporter{
		name:  "bad",
		cloud: "gcp",
		err:   errors.New("provider unavailable"),
	}

	agg := NewAggregator(AggregatorConfig{
		CollectInterval: 10 * time.Second,
		Exporters:       []CloudExporter{good, bad},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := agg.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	agg.collect(ctx)

	got := 0
	for {
		select {
		case <-agg.metrics:
			got++
		case <-time.After(50 * time.Millisecond):
			goto done
		}
	}
done:
	if got != 1 {
		t.Errorf("expected 1 metric (from good exporter), got %d", got)
	}
}

func TestAggregator_MetricBufferDropsWhenFull(t *testing.T) {
	ts := time.Now()
	// Buffer is 1000. Push 1010 metrics; final 10 should be silently dropped.
	large := make([]Metric, 1010)
	for i := range large {
		large[i] = Metric{Name: "m", Value: float64(i), Timestamp: ts, Cloud: "aws"}
	}

	exp := &mockExporter{name: "aws", cloud: "aws", metrics: large}
	agg := NewAggregator(AggregatorConfig{
		CollectInterval: 10 * time.Second,
		Exporters:       []CloudExporter{exp},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := agg.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Should not block or panic.
	agg.collect(ctx)
}

// ---- mockExporter behaviour tests ----

func TestMockExporter_InterfaceCompliance(t *testing.T) {
	var _ CloudExporter = &mockExporter{}
}

func TestMockExporter_CollectMetrics_ReturnsConfiguredMetrics(t *testing.T) {
	ts := time.Now()
	exp := &mockExporter{
		name:  "test",
		cloud: "aws",
		metrics: []Metric{
			{Name: "latency", Cloud: "aws", Value: 120.0, Timestamp: ts},
		},
	}

	got, err := exp.CollectMetrics(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(got))
	}
	if got[0].Name != "latency" {
		t.Errorf("unexpected metric name: %q", got[0].Name)
	}
}

func TestMockExporter_CollectMetrics_PropagatesError(t *testing.T) {
	exp := &mockExporter{
		name:  "err-exporter",
		cloud: "azure",
		err:   errors.New("auth failure"),
	}

	_, err := exp.CollectMetrics(context.Background())
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestMockExporter_CollectMetrics_EmptyOnNoMetrics(t *testing.T) {
	exp := &mockExporter{name: "empty", cloud: "gcp"}
	got, err := exp.CollectMetrics(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d metrics", len(got))
	}
}

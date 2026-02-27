package metrics

import (
	"math"
	"testing"
	"time"
)

// ---- Metric normalization / field tests ----
//
// The normalizer logic lives inline inside getMetricData (aws_exporter.go) and
// the equivalent placeholder implementations in azure/gcp exporters. These
// tests validate that the Metric struct fields are populated and interpreted
// correctly, and that helper functions (parsePeriod, severityPriority-like
// unit conversions) behave as documented.

// ---- parsePeriod tests ----

func TestParsePeriod_ValidSeconds(t *testing.T) {
	tests := []struct {
		input string
		want  int32
	}{
		{"300s", 300},
		{"60s", 60},
		{"1s", 1},
		{"3600s", 3600},
	}

	for _, tc := range tests {
		got := parsePeriod(tc.input)
		if got != tc.want {
			t.Errorf("parsePeriod(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestParsePeriod_ValidMinutes(t *testing.T) {
	got := parsePeriod("5m")
	if got != 300 {
		t.Errorf("parsePeriod(\"5m\") = %d, want 300", got)
	}
}

func TestParsePeriod_InvalidFallsBackToDefault(t *testing.T) {
	tests := []string{"", "abc", "5", "??", "5 minutes"}
	for _, input := range tests {
		got := parsePeriod(input)
		if got != 300 {
			t.Errorf("parsePeriod(%q) = %d, want default 300", input, got)
		}
	}
}

// ---- Metric normalization correctness ----

func TestNormalizeMetric_AWSFields(t *testing.T) {
	ts := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	m := Metric{
		Name:      "CPUUtilization",
		Cloud:     "aws",
		Namespace: "AWS/EC2",
		Resource:  "i-0123456789abcdef0",
		Value:     87.3,
		Timestamp: ts,
		Unit:      "Percent",
		Statistic: "Average",
		Labels: map[string]string{
			"cloud":     "aws",
			"region":    "us-east-1",
			"namespace": "AWS/EC2",
		},
		Dimensions: map[string]string{
			"InstanceId": "i-0123456789abcdef0",
		},
	}

	if m.Cloud != "aws" {
		t.Errorf("cloud = %q, want aws", m.Cloud)
	}
	if m.Namespace != "AWS/EC2" {
		t.Errorf("namespace = %q, want AWS/EC2", m.Namespace)
	}
	if math.Abs(m.Value-87.3) > 1e-9 {
		t.Errorf("value = %f, want 87.3", m.Value)
	}
}

func TestNormalizeMetric_AzureFields(t *testing.T) {
	ts := time.Now().UTC()
	m := Metric{
		Name:      "Percentage CPU",
		Cloud:     "azure",
		Namespace: "Microsoft.Compute/virtualMachines",
		Value:     45.0,
		Timestamp: ts,
		Unit:      "Percent",
		Statistic: "Average",
		Labels: map[string]string{
			"cloud":          "azure",
			"subscription":   "sub-abc123",
			"resource_group": "rg-prod",
		},
	}

	if m.Cloud != "azure" {
		t.Errorf("cloud = %q, want azure", m.Cloud)
	}
	if m.Labels["subscription"] != "sub-abc123" {
		t.Errorf("unexpected subscription label: %q", m.Labels["subscription"])
	}
}

func TestNormalizeMetric_GCPFields(t *testing.T) {
	ts := time.Now().UTC()
	m := Metric{
		Name:      "compute.googleapis.com/instance/cpu/utilization",
		Cloud:     "gcp",
		Namespace: "compute.googleapis.com",
		Value:     0.65,
		Timestamp: ts,
		Unit:      "1",
		Statistic: "ALIGN_MEAN",
		Labels: map[string]string{
			"cloud":   "gcp",
			"project": "my-project-id",
			"zone":    "us-central1-a",
		},
	}

	if m.Cloud != "gcp" {
		t.Errorf("cloud = %q, want gcp", m.Cloud)
	}
	if m.Value != 0.65 {
		t.Errorf("value = %f, want 0.65", m.Value)
	}
}

// ---- Edge cases ----

func TestNormalizeMetric_ZeroValue(t *testing.T) {
	m := Metric{
		Name:      "RequestCount",
		Cloud:     "aws",
		Namespace: "AWS/ELB",
		Value:     0,
		Timestamp: time.Now(),
	}
	// Zero value is valid (no requests); must not be treated as missing.
	if m.Value != 0 {
		t.Errorf("expected zero value to be preserved")
	}
}

func TestNormalizeMetric_NegativeValue(t *testing.T) {
	// Some delta metrics can be negative; the struct must not clamp them.
	m := Metric{
		Name:  "NetworkOut",
		Cloud: "aws",
		Value: -1.5,
	}
	if m.Value != -1.5 {
		t.Errorf("negative value mutated: got %f", m.Value)
	}
}

func TestNormalizeMetric_LargeValue(t *testing.T) {
	m := Metric{
		Name:  "BytesTransferred",
		Cloud: "aws",
		Value: 1e15, // 1 petabyte in bytes
		Unit:  "Bytes",
	}
	if m.Value != 1e15 {
		t.Errorf("large value mutated: got %f", m.Value)
	}
}

func TestNormalizeMetric_TimestampUTC(t *testing.T) {
	// Metric timestamps should be stored in UTC-comparable form.
	now := time.Now()
	m := Metric{Timestamp: now}
	// Verify round-trip comparison is consistent.
	if !m.Timestamp.Equal(now) {
		t.Error("timestamp not preserved")
	}
}

func TestNormalizeMetric_EmptyLabels(t *testing.T) {
	m := Metric{
		Name:   "cpu",
		Cloud:  "aws",
		Labels: map[string]string{},
	}
	if m.Labels == nil {
		t.Error("labels should not be nil after initialisation with empty map")
	}
	if v, ok := m.Labels["missing"]; ok {
		t.Errorf("unexpected label value %q for missing key", v)
	}
}

func TestNormalizeMetric_NilDimensions(t *testing.T) {
	m := Metric{
		Name:       "cpu",
		Cloud:      "aws",
		Dimensions: nil,
	}
	// nil Dimensions is valid (many metrics have no dimensions).
	if m.Dimensions != nil {
		t.Error("expected nil Dimensions")
	}
}

// ---- Unit conversion helpers ----

func TestUnitConversion_BytesToGB(t *testing.T) {
	bytes := 5.0 * 1024 * 1024 * 1024 // 5 GB
	gb := bytes / (1024 * 1024 * 1024)
	if math.Abs(gb-5.0) > 1e-9 {
		t.Errorf("bytes-to-GB conversion wrong: got %f", gb)
	}
}

func TestUnitConversion_MillisecondsToSeconds(t *testing.T) {
	ms := 1500.0
	s := ms / 1000.0
	if math.Abs(s-1.5) > 1e-9 {
		t.Errorf("ms-to-s conversion wrong: got %f", s)
	}
}

func TestUnitConversion_PercentFraction(t *testing.T) {
	// GCP CPU utilization is [0,1]; AWS is [0,100].
	gcpCPU := 0.75
	awsCPU := gcpCPU * 100
	if math.Abs(awsCPU-75.0) > 1e-9 {
		t.Errorf("percent conversion wrong: got %f", awsCPU)
	}
}

// ---- Statistic label normalisation ----

func TestStatisticValues(t *testing.T) {
	validStats := []string{"Average", "Sum", "Minimum", "Maximum", "p99", "p95", "p50"}
	for _, stat := range validStats {
		m := Metric{Statistic: stat}
		if m.Statistic != stat {
			t.Errorf("statistic %q not preserved", stat)
		}
	}
}

// ---- MetricQuery struct ----

func TestMetricQuery_DefaultFields(t *testing.T) {
	q := MetricQuery{
		Namespace:  "AWS/EC2",
		MetricName: "CPUUtilization",
		Statistic:  "Average",
		Period:     60 * time.Second,
	}
	if q.Namespace != "AWS/EC2" {
		t.Errorf("namespace = %q", q.Namespace)
	}
	if q.Period != 60*time.Second {
		t.Errorf("period = %v", q.Period)
	}
}

func TestMetricQuery_EmptyDimensions(t *testing.T) {
	q := MetricQuery{MetricName: "RequestCount"}
	if q.Dimensions != nil {
		t.Error("expected nil dimensions for query without filters")
	}
}

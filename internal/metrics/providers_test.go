package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/lvonguyen/multicloud-observability/internal/otel"
)

// ---- Azure Monitor exporter tests ----

func TestAzureMonitorExporter_Name(t *testing.T) {
	exp, err := NewAzureMonitorExporter(context.Background(), AzureExporterConfig{
		SubscriptionID: "sub-test-001",
	})
	if err != nil {
		t.Fatalf("NewAzureMonitorExporter: %v", err)
	}
	if exp.Name() != "azure-monitor" {
		t.Errorf("Name() = %q, want azure-monitor", exp.Name())
	}
}

func TestAzureMonitorExporter_Cloud(t *testing.T) {
	exp, err := NewAzureMonitorExporter(context.Background(), AzureExporterConfig{
		SubscriptionID: "sub-test-001",
	})
	if err != nil {
		t.Fatalf("NewAzureMonitorExporter: %v", err)
	}
	if exp.Cloud() != "azure" {
		t.Errorf("Cloud() = %q, want azure", exp.Cloud())
	}
}

func TestAzureMonitorExporter_DefaultPollInterval(t *testing.T) {
	exp, err := NewAzureMonitorExporter(context.Background(), AzureExporterConfig{
		SubscriptionID: "sub-test-001",
	})
	if err != nil {
		t.Fatalf("NewAzureMonitorExporter: %v", err)
	}
	if exp.config.PollInterval != 60*time.Second {
		t.Errorf("PollInterval = %v, want 60s", exp.config.PollInterval)
	}
}

func TestAzureMonitorExporter_CustomPollInterval(t *testing.T) {
	exp, err := NewAzureMonitorExporter(context.Background(), AzureExporterConfig{
		SubscriptionID: "sub-test-001",
		PollInterval:   30 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewAzureMonitorExporter: %v", err)
	}
	if exp.config.PollInterval != 30*time.Second {
		t.Errorf("PollInterval = %v, want 30s", exp.config.PollInterval)
	}
}

func TestAzureMonitorExporter_Start(t *testing.T) {
	exp, err := NewAzureMonitorExporter(context.Background(), AzureExporterConfig{
		SubscriptionID: "sub-test-001",
	})
	if err != nil {
		t.Fatalf("NewAzureMonitorExporter: %v", err)
	}
	if err := exp.Start(context.Background()); err != nil {
		t.Errorf("Start() error: %v", err)
	}
}

func TestAzureMonitorExporter_Stop(t *testing.T) {
	exp, err := NewAzureMonitorExporter(context.Background(), AzureExporterConfig{
		SubscriptionID: "sub-test-001",
	})
	if err != nil {
		t.Fatalf("NewAzureMonitorExporter: %v", err)
	}
	if err := exp.Stop(); err != nil {
		t.Errorf("Stop() error: %v", err)
	}
}

func TestAzureMonitorExporter_CollectMetrics_ReturnsEmpty(t *testing.T) {
	exp, err := NewAzureMonitorExporter(context.Background(), AzureExporterConfig{
		SubscriptionID: "sub-test-001",
	})
	if err != nil {
		t.Fatalf("NewAzureMonitorExporter: %v", err)
	}
	got, err := exp.CollectMetrics(context.Background())
	if err != nil {
		t.Errorf("CollectMetrics() error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty metrics, got %d", len(got))
	}
}

func TestAzureMonitorExporter_CollectMetrics_ContextCancelled(t *testing.T) {
	exp, err := NewAzureMonitorExporter(context.Background(), AzureExporterConfig{
		SubscriptionID: "sub-test-001",
	})
	if err != nil {
		t.Fatalf("NewAzureMonitorExporter: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	// Should not block or panic; stub implementation returns empty slice.
	_, err = exp.CollectMetrics(ctx)
	if err != nil {
		t.Errorf("unexpected error on cancelled ctx: %v", err)
	}
}

func TestAzureMonitorExporter_InterfaceCompliance(t *testing.T) {
	exp, err := NewAzureMonitorExporter(context.Background(), AzureExporterConfig{
		SubscriptionID: "sub-test-001",
	})
	if err != nil {
		t.Fatalf("NewAzureMonitorExporter: %v", err)
	}
	var _ CloudExporter = exp
}

// ---- GCP Monitoring exporter tests ----

func TestGCPMonitoringExporter_Name(t *testing.T) {
	exp, err := NewGCPMonitoringExporter(context.Background(), GCPExporterConfig{
		ProjectID: "my-project",
	})
	if err != nil {
		t.Fatalf("NewGCPMonitoringExporter: %v", err)
	}
	if exp.Name() != "gcp-monitoring" {
		t.Errorf("Name() = %q, want gcp-monitoring", exp.Name())
	}
}

func TestGCPMonitoringExporter_Cloud(t *testing.T) {
	exp, err := NewGCPMonitoringExporter(context.Background(), GCPExporterConfig{
		ProjectID: "my-project",
	})
	if err != nil {
		t.Fatalf("NewGCPMonitoringExporter: %v", err)
	}
	if exp.Cloud() != "gcp" {
		t.Errorf("Cloud() = %q, want gcp", exp.Cloud())
	}
}

func TestGCPMonitoringExporter_DefaultPollInterval(t *testing.T) {
	exp, err := NewGCPMonitoringExporter(context.Background(), GCPExporterConfig{
		ProjectID: "my-project",
	})
	if err != nil {
		t.Fatalf("NewGCPMonitoringExporter: %v", err)
	}
	if exp.config.PollInterval != 60*time.Second {
		t.Errorf("PollInterval = %v, want 60s", exp.config.PollInterval)
	}
}

func TestGCPMonitoringExporter_CustomPollInterval(t *testing.T) {
	exp, err := NewGCPMonitoringExporter(context.Background(), GCPExporterConfig{
		ProjectID:    "my-project",
		PollInterval: 120 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewGCPMonitoringExporter: %v", err)
	}
	if exp.config.PollInterval != 120*time.Second {
		t.Errorf("PollInterval = %v, want 120s", exp.config.PollInterval)
	}
}

func TestGCPMonitoringExporter_Start(t *testing.T) {
	exp, err := NewGCPMonitoringExporter(context.Background(), GCPExporterConfig{
		ProjectID: "my-project",
	})
	if err != nil {
		t.Fatalf("NewGCPMonitoringExporter: %v", err)
	}
	if err := exp.Start(context.Background()); err != nil {
		t.Errorf("Start() error: %v", err)
	}
}

func TestGCPMonitoringExporter_Stop(t *testing.T) {
	exp, err := NewGCPMonitoringExporter(context.Background(), GCPExporterConfig{
		ProjectID: "my-project",
	})
	if err != nil {
		t.Fatalf("NewGCPMonitoringExporter: %v", err)
	}
	if err := exp.Stop(); err != nil {
		t.Errorf("Stop() error: %v", err)
	}
}

func TestGCPMonitoringExporter_CollectMetrics_ReturnsEmpty(t *testing.T) {
	exp, err := NewGCPMonitoringExporter(context.Background(), GCPExporterConfig{
		ProjectID: "my-project",
	})
	if err != nil {
		t.Fatalf("NewGCPMonitoringExporter: %v", err)
	}
	got, err := exp.CollectMetrics(context.Background())
	if err != nil {
		t.Errorf("CollectMetrics() error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty metrics, got %d", len(got))
	}
}

func TestGCPMonitoringExporter_CollectMetrics_ContextCancelled(t *testing.T) {
	exp, err := NewGCPMonitoringExporter(context.Background(), GCPExporterConfig{
		ProjectID: "my-project",
	})
	if err != nil {
		t.Fatalf("NewGCPMonitoringExporter: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = exp.CollectMetrics(ctx)
	if err != nil {
		t.Errorf("unexpected error on cancelled ctx: %v", err)
	}
}

func TestGCPMonitoringExporter_WithMetricTypes(t *testing.T) {
	exp, err := NewGCPMonitoringExporter(context.Background(), GCPExporterConfig{
		ProjectID: "my-project",
		MetricTypes: []otel.GCPMetricSettings{
			{Type: "compute.googleapis.com/instance/cpu/utilization"},
			{Type: "compute.googleapis.com/instance/memory/balloon/ram_used"},
		},
	})
	if err != nil {
		t.Fatalf("NewGCPMonitoringExporter: %v", err)
	}
	if len(exp.config.MetricTypes) != 2 {
		t.Errorf("expected 2 metric types, got %d", len(exp.config.MetricTypes))
	}
}

func TestGCPMonitoringExporter_InterfaceCompliance(t *testing.T) {
	exp, err := NewGCPMonitoringExporter(context.Background(), GCPExporterConfig{
		ProjectID: "my-project",
	})
	if err != nil {
		t.Fatalf("NewGCPMonitoringExporter: %v", err)
	}
	var _ CloudExporter = exp
}

// ---- defaultAWSNamespaces tests ----

func TestDefaultAWSNamespaces_NotEmpty(t *testing.T) {
	ns := defaultAWSNamespaces()
	if len(ns) == 0 {
		t.Error("defaultAWSNamespaces() returned empty slice")
	}
}

func TestDefaultAWSNamespaces_ContainsEC2CPU(t *testing.T) {
	ns := defaultAWSNamespaces()
	found := false
	for _, n := range ns {
		if n.Namespace == "AWS/EC2" && n.MetricName == "CPUUtilization" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected AWS/EC2:CPUUtilization in default namespaces")
	}
}

func TestDefaultAWSNamespaces_AllHavePeriod(t *testing.T) {
	ns := defaultAWSNamespaces()
	for _, n := range ns {
		if n.Period == "" {
			t.Errorf("namespace %q:%q has empty period", n.Namespace, n.MetricName)
		}
	}
}

func TestDefaultAWSNamespaces_AllHaveStat(t *testing.T) {
	ns := defaultAWSNamespaces()
	for _, n := range ns {
		if n.Stat == "" {
			t.Errorf("namespace %q:%q has empty stat", n.Namespace, n.MetricName)
		}
	}
}

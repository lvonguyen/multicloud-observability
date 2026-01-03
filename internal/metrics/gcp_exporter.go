package metrics

import (
	"context"
	"time"

	"github.com/lvonguyen/multicloud-observability/internal/otel"
)

// GCPExporterConfig holds configuration for GCP Cloud Monitoring exporter
type GCPExporterConfig struct {
	ProjectID    string
	MetricTypes  []otel.GCPMetricSettings
	PollInterval time.Duration
}

// GCPMonitoringExporter collects metrics from GCP Cloud Monitoring
type GCPMonitoringExporter struct {
	config GCPExporterConfig
	done   chan struct{}
}

// NewGCPMonitoringExporter creates a new GCP Cloud Monitoring exporter
func NewGCPMonitoringExporter(ctx context.Context, cfg GCPExporterConfig) (*GCPMonitoringExporter, error) {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 60 * time.Second
	}

	return &GCPMonitoringExporter{
		config: cfg,
		done:   make(chan struct{}),
	}, nil
}

// Name returns the exporter name
func (e *GCPMonitoringExporter) Name() string {
	return "gcp-monitoring"
}

// Cloud returns the cloud provider
func (e *GCPMonitoringExporter) Cloud() string {
	return "gcp"
}

// Start begins periodic metric collection
func (e *GCPMonitoringExporter) Start(ctx context.Context) error {
	return nil
}

// Stop halts metric collection
func (e *GCPMonitoringExporter) Stop() error {
	close(e.done)
	return nil
}

// CollectMetrics fetches metrics from GCP Cloud Monitoring
func (e *GCPMonitoringExporter) CollectMetrics(ctx context.Context) ([]Metric, error) {
	// In production: Use GCP Monitoring SDK
	// cloud.google.com/go/monitoring/apiv3/v2
	return []Metric{}, nil
}


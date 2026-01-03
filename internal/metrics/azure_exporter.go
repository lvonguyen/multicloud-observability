package metrics

import (
	"context"
	"time"
)

// AzureExporterConfig holds configuration for Azure Monitor exporter
type AzureExporterConfig struct {
	SubscriptionID string
	ResourceGroups []string
	PollInterval   time.Duration
}

// AzureMonitorExporter collects metrics from Azure Monitor
type AzureMonitorExporter struct {
	config AzureExporterConfig
	done   chan struct{}
}

// NewAzureMonitorExporter creates a new Azure Monitor exporter
func NewAzureMonitorExporter(ctx context.Context, cfg AzureExporterConfig) (*AzureMonitorExporter, error) {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 60 * time.Second
	}

	return &AzureMonitorExporter{
		config: cfg,
		done:   make(chan struct{}),
	}, nil
}

// Name returns the exporter name
func (e *AzureMonitorExporter) Name() string {
	return "azure-monitor"
}

// Cloud returns the cloud provider
func (e *AzureMonitorExporter) Cloud() string {
	return "azure"
}

// Start begins periodic metric collection
func (e *AzureMonitorExporter) Start(ctx context.Context) error {
	return nil
}

// Stop halts metric collection
func (e *AzureMonitorExporter) Stop() error {
	close(e.done)
	return nil
}

// CollectMetrics fetches metrics from Azure Monitor
func (e *AzureMonitorExporter) CollectMetrics(ctx context.Context) ([]Metric, error) {
	// In production: Use Azure Monitor SDK to query metrics
	// github.com/Azure/azure-sdk-for-go/sdk/monitor/azquery
	return []Metric{}, nil
}


// Package metrics provides multi-cloud metrics collection and aggregation.
package metrics

import (
	"context"
	"sync"
	"time"
)

// CloudExporter defines the interface for cloud-specific metric exporters
type CloudExporter interface {
	// Name returns the exporter name (e.g., "aws-cloudwatch")
	Name() string

	// Cloud returns the cloud provider (aws, azure, gcp)
	Cloud() string

	// CollectMetrics fetches metrics from the cloud provider
	CollectMetrics(ctx context.Context) ([]Metric, error)

	// Start begins periodic metric collection
	Start(ctx context.Context) error

	// Stop halts metric collection
	Stop() error
}

// Metric represents a normalized cloud metric
type Metric struct {
	// Identification
	Name      string            `json:"name"`
	Labels    map[string]string `json:"labels"`
	Cloud     string            `json:"cloud"`     // aws, azure, gcp
	Namespace string            `json:"namespace"` // Original namespace (e.g., AWS/EC2)
	Resource  string            `json:"resource"`  // Resource identifier

	// Value
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
	Unit      string    `json:"unit"`

	// Metadata
	Dimensions map[string]string `json:"dimensions"`
	Statistic  string            `json:"statistic"` // avg, sum, min, max, p99
}

// MetricQuery defines a metric to collect
type MetricQuery struct {
	Namespace  string            // Cloud namespace (e.g., AWS/EC2, Microsoft.Compute)
	MetricName string            // Metric name
	Dimensions map[string]string // Filter dimensions
	Statistic  string            // Aggregation (Average, Sum, p99)
	Period     time.Duration     // Collection period
}

// AggregatorConfig holds configuration for the metrics aggregator
type AggregatorConfig struct {
	PrometheusEndpoint string
	Exporters          []CloudExporter
	CollectInterval    time.Duration
}

// Aggregator collects metrics from multiple cloud exporters
type Aggregator struct {
	config    AggregatorConfig
	exporters []CloudExporter
	metrics   chan Metric
	done      chan struct{}
	stopOnce  sync.Once
}

// NewAggregator creates a new metrics aggregator
func NewAggregator(cfg AggregatorConfig) *Aggregator {
	if cfg.CollectInterval == 0 {
		cfg.CollectInterval = 60 * time.Second
	}

	return &Aggregator{
		config:    cfg,
		exporters: cfg.Exporters,
		metrics:   make(chan Metric, 1000),
		done:      make(chan struct{}),
	}
}

// Start begins collecting metrics from all exporters
func (a *Aggregator) Start(ctx context.Context) error {
	// Start each exporter
	for _, exp := range a.exporters {
		if err := exp.Start(ctx); err != nil {
			return err
		}
	}

	// Start collection loop
	go a.collectLoop(ctx)

	// Start Prometheus exposition
	go a.exposeMetrics(ctx)

	return nil
}

// collectLoop periodically collects metrics from all exporters
func (a *Aggregator) collectLoop(ctx context.Context) {
	ticker := time.NewTicker(a.config.CollectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.collect(ctx)
		}
	}
}

// collect fetches metrics from all exporters
func (a *Aggregator) collect(ctx context.Context) {
	for _, exp := range a.exporters {
		metrics, err := exp.CollectMetrics(ctx)
		if err != nil {
			// Log error but continue with other exporters
			continue
		}

		for _, m := range metrics {
			select {
			case a.metrics <- m:
			default:
				// Drop metric if buffer full
			}
		}
	}
}

// exposeMetrics serves metrics for Prometheus scraping
func (a *Aggregator) exposeMetrics(ctx context.Context) {
	// In production: use prometheus/client_golang to expose metrics
	// This would create Prometheus gauges/counters from collected metrics
}

// Shutdown gracefully stops the aggregator. Safe to call multiple times.
func (a *Aggregator) Shutdown(ctx context.Context) error {
	a.stopOnce.Do(func() {
		close(a.done)
	})

	for _, exp := range a.exporters {
		if err := exp.Stop(); err != nil {
			// Log but continue stopping others
		}
	}

	return nil
}

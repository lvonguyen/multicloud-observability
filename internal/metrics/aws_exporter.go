package metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/lvonguyen/multicloud-observability/internal/otel"
)

// AWSExporterConfig holds configuration for AWS CloudWatch exporter
type AWSExporterConfig struct {
	Region       string
	PollInterval time.Duration
	Namespaces   []otel.AWSNamespaceSettings
}


// AWSCloudWatchExporter collects metrics from AWS CloudWatch
type AWSCloudWatchExporter struct {
	config     AWSExporterConfig
	client     *cloudwatch.Client
	namespaces []otel.AWSNamespaceSettings
	done       chan struct{}
}

// NewAWSCloudWatchExporter creates a new AWS CloudWatch exporter
func NewAWSCloudWatchExporter(ctx context.Context, cfg AWSExporterConfig) (*AWSCloudWatchExporter, error) {
	// Load AWS config
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := cloudwatch.NewFromConfig(awsCfg)

	// Default namespaces if not specified
	if len(cfg.Namespaces) == 0 {
		cfg.Namespaces = defaultAWSNamespaces()
	}

	if cfg.PollInterval == 0 {
		cfg.PollInterval = 60 * time.Second
	}

	return &AWSCloudWatchExporter{
		config:     cfg,
		client:     client,
		namespaces: cfg.Namespaces,
		done:       make(chan struct{}),
	}, nil
}

// defaultAWSNamespaces returns commonly monitored AWS namespaces
func defaultAWSNamespaces() []otel.AWSNamespaceSettings {
	return []otel.AWSNamespaceSettings{
		{Namespace: "AWS/EC2", MetricName: "CPUUtilization", Stat: "Average", Period: "300s"},
		{Namespace: "AWS/Lambda", MetricName: "Duration", Stat: "p99", Period: "60s"},
		{Namespace: "AWS/RDS", MetricName: "CPUUtilization", Stat: "Average", Period: "300s"},
		{Namespace: "AWS/ELB", MetricName: "RequestCount", Stat: "Sum", Period: "60s"},
	}
}

// Name returns the exporter name
func (e *AWSCloudWatchExporter) Name() string {
	return "aws-cloudwatch"
}

// Cloud returns the cloud provider
func (e *AWSCloudWatchExporter) Cloud() string {
	return "aws"
}

// Start begins periodic metric collection
func (e *AWSCloudWatchExporter) Start(ctx context.Context) error {
	// Collection is handled by aggregator
	return nil
}

// Stop halts metric collection
func (e *AWSCloudWatchExporter) Stop() error {
	close(e.done)
	return nil
}

// CollectMetrics fetches metrics from CloudWatch
func (e *AWSCloudWatchExporter) CollectMetrics(ctx context.Context) ([]Metric, error) {
	var allMetrics []Metric
	endTime := time.Now()

	for _, ns := range e.namespaces {
		periodSeconds := parsePeriod(ns.Period)
		startTime := endTime.Add(-time.Duration(periodSeconds*2) * time.Second)

		metrics, err := e.getMetricData(ctx, ns, startTime, endTime, periodSeconds)
		if err != nil {
			// Log error but continue
			continue
		}
		allMetrics = append(allMetrics, metrics...)
	}

	return allMetrics, nil
}

// parsePeriod converts period string (e.g., "300s") to seconds
func parsePeriod(period string) int32 {
	d, err := time.ParseDuration(period)
	if err != nil {
		return 300 // default 5 minutes
	}
	return int32(d.Seconds())
}

// getMetricData fetches data for a specific metric
func (e *AWSCloudWatchExporter) getMetricData(ctx context.Context, ns otel.AWSNamespaceSettings, startTime, endTime time.Time, periodSeconds int32) ([]Metric, error) {
	// Build metric data query
	queries := []types.MetricDataQuery{
		{
			Id: aws.String("m1"),
			MetricStat: &types.MetricStat{
				Metric: &types.Metric{
					Namespace:  aws.String(ns.Namespace),
					MetricName: aws.String(ns.MetricName),
				},
				Period: aws.Int32(periodSeconds),
				Stat:   aws.String(ns.Stat),
			},
		},
	}

	input := &cloudwatch.GetMetricDataInput{
		MetricDataQueries: queries,
		StartTime:         aws.Time(startTime),
		EndTime:           aws.Time(endTime),
	}

	result, err := e.client.GetMetricData(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get metric data: %w", err)
	}

	var metrics []Metric
	for _, mr := range result.MetricDataResults {
		for i, ts := range mr.Timestamps {
			m := Metric{
				Name:      ns.MetricName,
				Cloud:     "aws",
				Namespace: ns.Namespace,
				Value:     mr.Values[i],
				Timestamp: ts,
				Statistic: ns.Stat,
				Labels: map[string]string{
					"cloud":     "aws",
					"region":    e.config.Region,
					"namespace": ns.Namespace,
				},
			}
			metrics = append(metrics, m)
		}
	}

	return metrics, nil
}


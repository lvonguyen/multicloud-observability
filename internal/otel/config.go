// Package otel provides OpenTelemetry collector configuration and processing.
package otel

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the OpenTelemetry Collector configuration
type Config struct {
	Receivers  ReceiversConfig  `yaml:"receivers"`
	Processors ProcessorsConfig `yaml:"processors"`
	Exporters  ExportersConfig  `yaml:"exporters"`
	Service    ServiceConfig    `yaml:"service"`
}

// ReceiversConfig holds receiver configurations
type ReceiversConfig struct {
	AWS   AWSReceiverConfig   `yaml:"awscloudwatch"`
	Azure AzureReceiverConfig `yaml:"azuremonitor"`
	GCP   GCPReceiverConfig   `yaml:"googlecloudmonitoring"`
	OTLP  OTLPReceiverConfig  `yaml:"otlp"`
}

// AWSReceiverConfig holds AWS CloudWatch receiver settings
type AWSReceiverConfig struct {
	Region       string                 `yaml:"region"`
	PollInterval time.Duration          `yaml:"poll_interval"`
	Namespaces   []AWSNamespaceSettings `yaml:"metrics"`
}

// AWSNamespaceSettings defines metrics to collect from an AWS namespace
type AWSNamespaceSettings struct {
	Namespace  string   `yaml:"namespace"`
	MetricName string   `yaml:"metric_name"`
	Period     string   `yaml:"period"`
	Stat       string   `yaml:"stat"`
	Dimensions []string `yaml:"dimensions,omitempty"`
}

// AzureReceiverConfig holds Azure Monitor receiver settings
type AzureReceiverConfig struct {
	SubscriptionID string                  `yaml:"subscription_id"`
	ResourceGroups []string                `yaml:"resource_groups"`
	Metrics        []AzureMetricSettings   `yaml:"metrics"`
}

// AzureMetricSettings defines metrics to collect from Azure
type AzureMetricSettings struct {
	ResourceType string   `yaml:"resource_type"`
	MetricNames  []string `yaml:"metric_names"`
}

// GCPReceiverConfig holds GCP Cloud Monitoring receiver settings
type GCPReceiverConfig struct {
	ProjectID   string              `yaml:"project"`
	MetricTypes []GCPMetricSettings `yaml:"metrics"`
}

// GCPMetricSettings defines metrics to collect from GCP
type GCPMetricSettings struct {
	Type   string `yaml:"type"`
	Filter string `yaml:"filter,omitempty"`
}

// OTLPReceiverConfig holds OTLP receiver settings
type OTLPReceiverConfig struct {
	Protocols OTLPProtocols `yaml:"protocols"`
}

// OTLPProtocols defines OTLP protocol settings
type OTLPProtocols struct {
	GRPC GRPCConfig `yaml:"grpc"`
	HTTP HTTPConfig `yaml:"http"`
}

// GRPCConfig holds gRPC settings
type GRPCConfig struct {
	Endpoint string `yaml:"endpoint"`
}

// HTTPConfig holds HTTP settings
type HTTPConfig struct {
	Endpoint string `yaml:"endpoint"`
}

// ProcessorsConfig holds processor configurations
type ProcessorsConfig struct {
	Batch    BatchProcessorConfig    `yaml:"batch"`
	Resource ResourceProcessorConfig `yaml:"resource"`
}

// BatchProcessorConfig holds batch processor settings
type BatchProcessorConfig struct {
	Timeout   string `yaml:"timeout"`
	SendBatch int    `yaml:"send_batch_size"`
}

// ResourceProcessorConfig holds resource processor settings
type ResourceProcessorConfig struct {
	Attributes []ResourceAttribute `yaml:"attributes"`
}

// ResourceAttribute defines a resource attribute modification
type ResourceAttribute struct {
	Key    string `yaml:"key"`
	Value  string `yaml:"value,omitempty"`
	Action string `yaml:"action"` // insert, update, upsert, delete
}

// ExportersConfig holds exporter configurations
type ExportersConfig struct {
	Prometheus PrometheusExporterConfig `yaml:"prometheus"`
	OTLP       OTLPExporterConfig       `yaml:"otlp"`
	Loki       LokiExporterConfig       `yaml:"loki"`
}

// PrometheusExporterConfig holds Prometheus exporter settings
type PrometheusExporterConfig struct {
	Endpoint string `yaml:"endpoint"`
}

// OTLPExporterConfig holds OTLP exporter settings
type OTLPExporterConfig struct {
	Endpoint string `yaml:"endpoint"`
}

// LokiExporterConfig holds Loki exporter settings
type LokiExporterConfig struct {
	Endpoint string `yaml:"endpoint"`
}

// ServiceConfig holds service pipeline configurations
type ServiceConfig struct {
	Pipelines PipelinesConfig `yaml:"pipelines"`
}

// PipelinesConfig holds pipeline definitions
type PipelinesConfig struct {
	Metrics PipelineConfig `yaml:"metrics"`
	Traces  PipelineConfig `yaml:"traces"`
	Logs    PipelineConfig `yaml:"logs"`
}

// PipelineConfig defines a telemetry pipeline
type PipelineConfig struct {
	Receivers  []string `yaml:"receivers"`
	Processors []string `yaml:"processors"`
	Exporters  []string `yaml:"exporters"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		// Return default config if file doesn't exist
		return DefaultConfig(), nil
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Receivers: ReceiversConfig{
			AWS: AWSReceiverConfig{
				Region:       "us-east-1",
				PollInterval: 60 * time.Second,
				Namespaces: []AWSNamespaceSettings{
					{Namespace: "AWS/EC2", MetricName: "CPUUtilization", Period: "60s", Stat: "Average"},
					{Namespace: "AWS/Lambda", MetricName: "Duration", Period: "60s", Stat: "p99"},
				},
			},
			GCP: GCPReceiverConfig{
				MetricTypes: []GCPMetricSettings{
					{Type: "compute.googleapis.com/instance/cpu/utilization"},
				},
			},
			OTLP: OTLPReceiverConfig{
				Protocols: OTLPProtocols{
					GRPC: GRPCConfig{Endpoint: "0.0.0.0:4317"},
					HTTP: HTTPConfig{Endpoint: "0.0.0.0:4318"},
				},
			},
		},
		Processors: ProcessorsConfig{
			Batch: BatchProcessorConfig{
				Timeout:   "1s",
				SendBatch: 1000,
			},
		},
		Exporters: ExportersConfig{
			Prometheus: PrometheusExporterConfig{
				Endpoint: "0.0.0.0:8889",
			},
			OTLP: OTLPExporterConfig{
				Endpoint: "tempo:4317",
			},
		},
		Service: ServiceConfig{
			Pipelines: PipelinesConfig{
				Metrics: PipelineConfig{
					Receivers:  []string{"awscloudwatch", "azuremonitor", "googlecloudmonitoring", "otlp"},
					Processors: []string{"batch"},
					Exporters:  []string{"prometheus"},
				},
				Traces: PipelineConfig{
					Receivers:  []string{"otlp"},
					Processors: []string{"batch"},
					Exporters:  []string{"otlp"},
				},
			},
		},
	}
}


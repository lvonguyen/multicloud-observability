package otel

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---- LoadConfig tests ----

func TestLoadConfig_MissingFile_ReturnsDefault(t *testing.T) {
	cfg, err := LoadConfig("/tmp/nonexistent-mco-config-12345.yaml")
	if err != nil {
		t.Fatalf("LoadConfig with missing file should return default, got error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
}

func TestLoadConfig_ValidYAML(t *testing.T) {
	yaml := `
receivers:
  awscloudwatch:
    region: us-west-2
    poll_interval: 30s
  otlp:
    protocols:
      grpc:
        endpoint: "0.0.0.0:4317"
      http:
        endpoint: "0.0.0.0:4318"
processors:
  batch:
    timeout: "2s"
    send_batch_size: 500
exporters:
  prometheus:
    endpoint: "0.0.0.0:9090"
service:
  pipelines:
    metrics:
      receivers: [awscloudwatch, otlp]
      processors: [batch]
      exporters: [prometheus]
`
	path := writeTempConfig(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Receivers.AWS.Region != "us-west-2" {
		t.Errorf("region = %q, want us-west-2", cfg.Receivers.AWS.Region)
	}
	if cfg.Exporters.Prometheus.Endpoint != "0.0.0.0:9090" {
		t.Errorf("prometheus endpoint = %q, want 0.0.0.0:9090", cfg.Exporters.Prometheus.Endpoint)
	}
}

func TestLoadConfig_InvalidYAML_ReturnsError(t *testing.T) {
	path := writeTempConfig(t, ":: invalid: yaml: {{{")
	_, err := LoadConfig(path)
	if err == nil {
		t.Error("expected parse error for invalid YAML, got nil")
	}
}

func TestLoadConfig_EmptyFile_ReturnsZeroConfig(t *testing.T) {
	path := writeTempConfig(t, "")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig empty file: %v", err)
	}
	// Empty YAML produces zero-value config (all fields empty/zero).
	if cfg.Receivers.AWS.Region != "" {
		t.Errorf("expected empty region for empty config, got %q", cfg.Receivers.AWS.Region)
	}
}

func TestLoadConfig_PipelineReceivers(t *testing.T) {
	yaml := `
service:
  pipelines:
    metrics:
      receivers: [awscloudwatch, azuremonitor, googlecloudmonitoring]
      processors: [batch]
      exporters: [prometheus]
`
	path := writeTempConfig(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.Service.Pipelines.Metrics.Receivers) != 3 {
		t.Errorf("expected 3 receivers, got %d", len(cfg.Service.Pipelines.Metrics.Receivers))
	}
}

func TestLoadConfig_AWSNamespaces(t *testing.T) {
	yaml := `
receivers:
  awscloudwatch:
    region: us-east-1
    metrics:
      - namespace: AWS/EC2
        metric_name: CPUUtilization
        period: "60s"
        stat: Average
      - namespace: AWS/Lambda
        metric_name: Duration
        period: "60s"
        stat: p99
`
	path := writeTempConfig(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.Receivers.AWS.Namespaces) != 2 {
		t.Errorf("expected 2 AWS namespaces, got %d", len(cfg.Receivers.AWS.Namespaces))
	}
	if cfg.Receivers.AWS.Namespaces[0].MetricName != "CPUUtilization" {
		t.Errorf("unexpected metric name: %q", cfg.Receivers.AWS.Namespaces[0].MetricName)
	}
}

// ---- DefaultConfig tests ----

func TestDefaultConfig_NotNil(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}
}

func TestDefaultConfig_AWSRegion(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Receivers.AWS.Region != "us-east-1" {
		t.Errorf("default AWS region = %q, want us-east-1", cfg.Receivers.AWS.Region)
	}
}

func TestDefaultConfig_AWSPollInterval(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Receivers.AWS.PollInterval != 60*time.Second {
		t.Errorf("AWS poll interval = %v, want 60s", cfg.Receivers.AWS.PollInterval)
	}
}

func TestDefaultConfig_AWSNamespaces_NotEmpty(t *testing.T) {
	cfg := DefaultConfig()
	if len(cfg.Receivers.AWS.Namespaces) == 0 {
		t.Error("expected default AWS namespaces, got none")
	}
}

func TestDefaultConfig_AWSNamespaces_ContainEC2(t *testing.T) {
	cfg := DefaultConfig()
	found := false
	for _, ns := range cfg.Receivers.AWS.Namespaces {
		if ns.Namespace == "AWS/EC2" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected AWS/EC2 in default namespaces")
	}
}

func TestDefaultConfig_GCPMetricTypes_NotEmpty(t *testing.T) {
	cfg := DefaultConfig()
	if len(cfg.Receivers.GCP.MetricTypes) == 0 {
		t.Error("expected default GCP metric types, got none")
	}
}

func TestDefaultConfig_OTLPEndpoints(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Receivers.OTLP.Protocols.GRPC.Endpoint == "" {
		t.Error("expected non-empty GRPC endpoint")
	}
	if cfg.Receivers.OTLP.Protocols.HTTP.Endpoint == "" {
		t.Error("expected non-empty HTTP endpoint")
	}
}

func TestDefaultConfig_BatchProcessorSendBatch(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Processors.Batch.SendBatch <= 0 {
		t.Errorf("batch send size = %d, want >0", cfg.Processors.Batch.SendBatch)
	}
}

func TestDefaultConfig_PrometheusEndpoint(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Exporters.Prometheus.Endpoint == "" {
		t.Error("expected non-empty Prometheus endpoint")
	}
}

func TestDefaultConfig_OTLPExporterEndpoint(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Exporters.OTLP.Endpoint == "" {
		t.Error("expected non-empty OTLP exporter endpoint")
	}
}

func TestDefaultConfig_MetricsPipeline(t *testing.T) {
	cfg := DefaultConfig()
	p := cfg.Service.Pipelines.Metrics
	if len(p.Receivers) == 0 {
		t.Error("metrics pipeline has no receivers")
	}
	if len(p.Exporters) == 0 {
		t.Error("metrics pipeline has no exporters")
	}
}

func TestDefaultConfig_TracesPipeline(t *testing.T) {
	cfg := DefaultConfig()
	p := cfg.Service.Pipelines.Traces
	if len(p.Receivers) == 0 {
		t.Error("traces pipeline has no receivers")
	}
}

// ---- Struct field validation ----

func TestAWSNamespaceSettings_Fields(t *testing.T) {
	ns := AWSNamespaceSettings{
		Namespace:  "AWS/EC2",
		MetricName: "CPUUtilization",
		Period:     "300s",
		Stat:       "Average",
		Dimensions: []string{"InstanceId"},
	}
	if ns.Namespace != "AWS/EC2" {
		t.Errorf("namespace = %q", ns.Namespace)
	}
	if len(ns.Dimensions) != 1 {
		t.Errorf("expected 1 dimension, got %d", len(ns.Dimensions))
	}
}

func TestGCPMetricSettings_WithFilter(t *testing.T) {
	ms := GCPMetricSettings{
		Type:   "compute.googleapis.com/instance/cpu/utilization",
		Filter: `resource.type="gce_instance"`,
	}
	if ms.Filter == "" {
		t.Error("expected non-empty filter")
	}
}

func TestGCPMetricSettings_WithoutFilter(t *testing.T) {
	ms := GCPMetricSettings{
		Type: "compute.googleapis.com/instance/cpu/utilization",
	}
	if ms.Type == "" {
		t.Error("metric type should not be empty")
	}
}

func TestResourceAttribute_Actions(t *testing.T) {
	actions := []string{"insert", "update", "upsert", "delete"}
	for _, action := range actions {
		ra := ResourceAttribute{Key: "env", Value: "prod", Action: action}
		if ra.Action != action {
			t.Errorf("action = %q, want %q", ra.Action, action)
		}
	}
}

func TestOTLPProtocols_Endpoints(t *testing.T) {
	p := OTLPProtocols{
		GRPC: GRPCConfig{Endpoint: "0.0.0.0:4317"},
		HTTP: HTTPConfig{Endpoint: "0.0.0.0:4318"},
	}
	if p.GRPC.Endpoint != "0.0.0.0:4317" {
		t.Errorf("grpc endpoint = %q", p.GRPC.Endpoint)
	}
	if p.HTTP.Endpoint != "0.0.0.0:4318" {
		t.Errorf("http endpoint = %q", p.HTTP.Endpoint)
	}
}

func TestPipelineConfig_MultipleReceivers(t *testing.T) {
	p := PipelineConfig{
		Receivers:  []string{"awscloudwatch", "azuremonitor", "googlecloudmonitoring", "otlp"},
		Processors: []string{"batch"},
		Exporters:  []string{"prometheus"},
	}
	if len(p.Receivers) != 4 {
		t.Errorf("expected 4 receivers, got %d", len(p.Receivers))
	}
}

// ---- Helpers ----

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writeTempConfig: %v", err)
	}
	return path
}

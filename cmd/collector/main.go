// Package main provides a custom OpenTelemetry Collector with multi-cloud receivers.
// This collector aggregates metrics, traces, and logs from AWS, Azure, and GCP
// into a unified observability pipeline.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/lvonguyen/multicloud-observability/internal/metrics"
	"github.com/lvonguyen/multicloud-observability/internal/otel"
)

// Config holds collector configuration
type Config struct {
	ConfigPath string
	LogLevel   string

	// Cloud credentials
	AWSRegion          string
	AzureSubscription  string
	GCPProject         string

	// Export targets
	PrometheusEndpoint string
	TempoEndpoint      string
	LokiEndpoint       string
}

func main() {
	cfg := parseFlags()

	// Initialize logger
	var logger *zap.Logger
	var err error
	if cfg.LogLevel == "debug" {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Starting Multi-Cloud Observability Collector",
		zap.String("config", cfg.ConfigPath),
		zap.String("aws_region", cfg.AWSRegion),
		zap.String("gcp_project", cfg.GCPProject),
	)

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
		cancel()
	}()

	// Load collector configuration
	collectorConfig, err := otel.LoadConfig(cfg.ConfigPath)
	if err != nil {
		logger.Fatal("Failed to load collector config", zap.Error(err))
	}

	// Initialize cloud metric exporters
	exporters := []metrics.CloudExporter{}

	// AWS CloudWatch Exporter
	if cfg.AWSRegion != "" {
		awsExporter, err := metrics.NewAWSCloudWatchExporter(ctx, metrics.AWSExporterConfig{
			Region:       cfg.AWSRegion,
			PollInterval: collectorConfig.Receivers.AWS.PollInterval,
			Namespaces:   collectorConfig.Receivers.AWS.Namespaces,
		})
		if err != nil {
			logger.Warn("Failed to initialize AWS exporter", zap.Error(err))
		} else {
			exporters = append(exporters, awsExporter)
			logger.Info("AWS CloudWatch exporter initialized", zap.String("region", cfg.AWSRegion))
		}
	}

	// Azure Monitor Exporter
	if cfg.AzureSubscription != "" {
		azureExporter, err := metrics.NewAzureMonitorExporter(ctx, metrics.AzureExporterConfig{
			SubscriptionID: cfg.AzureSubscription,
			ResourceGroups: collectorConfig.Receivers.Azure.ResourceGroups,
		})
		if err != nil {
			logger.Warn("Failed to initialize Azure exporter", zap.Error(err))
		} else {
			exporters = append(exporters, azureExporter)
			logger.Info("Azure Monitor exporter initialized", zap.String("subscription", cfg.AzureSubscription))
		}
	}

	// GCP Cloud Monitoring Exporter
	if cfg.GCPProject != "" {
		gcpExporter, err := metrics.NewGCPMonitoringExporter(ctx, metrics.GCPExporterConfig{
			ProjectID:   cfg.GCPProject,
			MetricTypes: collectorConfig.Receivers.GCP.MetricTypes,
		})
		if err != nil {
			logger.Warn("Failed to initialize GCP exporter", zap.Error(err))
		} else {
			exporters = append(exporters, gcpExporter)
			logger.Info("GCP Monitoring exporter initialized", zap.String("project", cfg.GCPProject))
		}
	}

	if len(exporters) == 0 {
		logger.Fatal("No cloud exporters configured. Set AWS_REGION, AZURE_SUBSCRIPTION_ID, or GCP_PROJECT_ID")
	}

	// Create metrics aggregator
	aggregator := metrics.NewAggregator(metrics.AggregatorConfig{
		PrometheusEndpoint: cfg.PrometheusEndpoint,
		Exporters:          exporters,
	})

	// Start metric collection
	if err := aggregator.Start(ctx); err != nil {
		logger.Fatal("Failed to start aggregator", zap.Error(err))
	}

	// Wait for shutdown
	<-ctx.Done()

	// Graceful shutdown
	logger.Info("Shutting down collector...")
	if err := aggregator.Shutdown(context.Background()); err != nil {
		logger.Error("Error during shutdown", zap.Error(err))
	}

	logger.Info("Collector stopped")
}

func parseFlags() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.ConfigPath, "config", "configs/otel-collector.yaml", "Path to collector config")
	flag.StringVar(&cfg.LogLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.StringVar(&cfg.PrometheusEndpoint, "prometheus", "http://localhost:9090", "Prometheus remote write endpoint")
	flag.StringVar(&cfg.TempoEndpoint, "tempo", "localhost:4317", "Tempo OTLP endpoint")
	flag.StringVar(&cfg.LokiEndpoint, "loki", "http://localhost:3100", "Loki push endpoint")
	flag.Parse()

	// Load from environment
	if v := os.Getenv("AWS_REGION"); v != "" {
		cfg.AWSRegion = v
	}
	if v := os.Getenv("AZURE_SUBSCRIPTION_ID"); v != "" {
		cfg.AzureSubscription = v
	}
	if v := os.Getenv("GCP_PROJECT_ID"); v != "" {
		cfg.GCPProject = v
	}

	return cfg
}

// printBanner prints the startup banner
func printBanner() {
	banner := `
╔══════════════════════════════════════════════════════════════════╗
║           Multi-Cloud Observability Collector                    ║
║                                                                  ║
║  Unified metrics, traces, and logs across AWS, Azure, and GCP   ║
╚══════════════════════════════════════════════════════════════════╝
`
	fmt.Println(banner)
}


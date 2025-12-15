package flipkartelbprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"

	// This import path must match your go.mod module name + /internal/metadata
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/flipkartelbprocessor/internal/metadata"
)

// NewFactory creates a factory for the processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		// You can set default values here if you want
		// Endpoints: []string{"http://localhost:8080"},
	}
}

func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	oCfg := cfg.(*Config)
	return newProcessor(set.Logger, oCfg, nextConsumer)
}

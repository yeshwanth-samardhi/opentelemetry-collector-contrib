package flipkartelbprocessor

import (
	"errors"

	"go.opentelemetry.io/collector/component"
)

// Config defines the configuration for the processor.
type Config struct {
	// Endpoints is a list of URLs to query sequentially.
	// The placeholder {peer_name} will be replaced by the value of net.peer.name.
	// Example: ["http://metadata-service-1/vips/{peer_name}", "http://metadata-service-2/vips/{peer_name}"]
	Endpoints []string `mapstructure:"endpoints"`

	// ClientTimeout defines the timeout for the HTTP client (e.g., "500ms", "1s")
	// You might want to parse this into a time.Duration in Validate() or createProcessor
	ClientTimeout string `mapstructure:"client_timeout"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if len(c.Endpoints) == 0 {
		return errors.New("endpoints list cannot be empty")
	}
	return nil
}

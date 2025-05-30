package tuiexporter

import "go.opentelemetry.io/collector/component"

// Config defines configuration for TUI exporter.
type Config struct {
	FromJSONFile bool   `mapstructure:"from_json_file"`
	MCPAddr      string `mapstructure:"mcp_addr"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
/* This is not used because the exporter does not have any configuration
func (cfg *Config) Validate() error {
	return nil
}
*/

package icmpcheckreceiver

import (
	"time"

	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	Targets                        []Target `mapstructure:"targets"`
}

type Target struct {
	Target      string         `mapstructure:"target"`
	PingCount   *int           `mapstructure:"ping_count"`
	PingTimeout *time.Duration `mapstructure:"ping_timeout"`
}

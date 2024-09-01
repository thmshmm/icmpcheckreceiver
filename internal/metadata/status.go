package metadata

import (
	"go.opentelemetry.io/collector/component"
)

var (
	Type = component.MustNewType("icmpcheck")
)

const (
	MetricsStability = component.StabilityLevelDevelopment
)

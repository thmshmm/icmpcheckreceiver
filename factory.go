package icmpcheckreceiver

import (
	"context"
	"errors"
	"github.com/thmshmm/icmpcheckreceiver/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()

	return &Config{
		ControllerConfig: cfg,
		Targets:          []Target{},
	}
}

func createMetricsReceiver(
	ctx context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	receiverCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("config is not a valid icmpping receiver config")
	}

	opts := []scraperhelper.ScraperControllerOption{}

	icmpScraper, err := newScraper(set.Logger, receiverCfg.Targets)
	if err != nil {
		return nil, err
	}

	scraper, err := scraperhelper.NewScraper(metadata.Type.String(), icmpScraper.Scrape)
	if err != nil {
		return nil, err
	}

	opts = append(opts, scraperhelper.AddScraper(scraper))

	return scraperhelper.NewScraperControllerReceiver(&receiverCfg.ControllerConfig, set, nextConsumer, opts...)
}

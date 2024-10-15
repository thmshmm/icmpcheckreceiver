package icmpcheckreceiver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	probing "github.com/prometheus-community/pro-bing"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

const (
	ATTR_PEER_IP         = "net.peer.ip"
	ATTR_PEER_NAME       = "net.peer.name"
	DEFAULT_PING_COUNT   = 3
	DEFAULT_PING_TIMEOUT = 5 * time.Second
)

type packet struct {
	Timestamp time.Time
	*probing.Packet
}

type pingResult struct {
	Packets        []*packet
	Stats          *probing.Statistics
	StatsTimestamp time.Time
}

type scraper struct {
	logger  *zap.Logger
	targets []Target
}

func newScraper(logger *zap.Logger, targets []Target) (*scraper, error) {
	return &scraper{
		logger:  logger,
		targets: targets,
	}, nil
}

func (s *scraper) ID() pipeline.Signal {
	return pipeline.SignalMetrics
}

func (s *scraper) Scrape(ctx context.Context) (pmetric.Metrics, error) {
	metrics := pmetric.NewMetrics()
	scopeMetrics := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()

	rttMetric := scopeMetrics.AppendEmpty()
	rttMetric.SetName("ping.rtt")
	rttMetric.SetUnit("ms")
	rttMetricDataPoints := rttMetric.SetEmptyGauge().DataPoints()

	minRttMetric := scopeMetrics.AppendEmpty()
	minRttMetric.SetName("ping.rtt.min")
	minRttMetric.SetUnit("ms")
	minRttMetricDataPoints := minRttMetric.SetEmptyGauge().DataPoints()

	maxRttMetric := scopeMetrics.AppendEmpty()
	maxRttMetric.SetName("ping.rtt.max")
	maxRttMetric.SetUnit("ms")
	maxRttMetricDataPoints := maxRttMetric.SetEmptyGauge().DataPoints()

	avgRttMetric := scopeMetrics.AppendEmpty()
	avgRttMetric.SetName("ping.rtt.avg")
	avgRttMetric.SetUnit("ms")
	avgRttMetricDataPoints := avgRttMetric.SetEmptyGauge().DataPoints()

	stddevRttMetric := scopeMetrics.AppendEmpty()
	stddevRttMetric.SetName("ping.rtt.stddev")
	stddevRttMetric.SetUnit("ms")
	stddevRttMetricDataPoints := stddevRttMetric.SetEmptyGauge().DataPoints()

	lossRatioMetric := scopeMetrics.AppendEmpty()
	lossRatioMetric.SetName("ping.loss.ratio")
	lossRatioMetricDataPoints := lossRatioMetric.SetEmptyGauge().DataPoints()

	for _, target := range s.targets {
		pingRes, err := ping(target)
		if err != nil {
			var dnsErr *net.DNSError

			if errors.As(err, &dnsErr) {
				s.logger.Log(zap.WarnLevel, "skipping target", zap.Error(dnsErr))

				continue
			} else {
				return pmetric.NewMetrics(), fmt.Errorf("failed to execute pinger for target %q: %w", target.Target, err)
			}
		}

		for _, pkt := range pingRes.Packets {
			appendPacketDataPoint(rttMetricDataPoints, float64(pkt.Rtt.Nanoseconds())/1e6, pkt, pingRes.Stats)
		}

		appendStatsDataPoint(lossRatioMetricDataPoints, pingRes.Stats.PacketLoss/100., pingRes)
		appendStatsDataPoint(minRttMetricDataPoints, float64(pingRes.Stats.MinRtt)/1e6, pingRes)
		appendStatsDataPoint(maxRttMetricDataPoints, float64(pingRes.Stats.MaxRtt)/1e6, pingRes)
		appendStatsDataPoint(avgRttMetricDataPoints, float64(pingRes.Stats.AvgRtt)/1e6, pingRes)
		appendStatsDataPoint(stddevRttMetricDataPoints, float64(pingRes.Stats.StdDevRtt)/1e6, pingRes)
	}

	return metrics, nil
}

func appendPacketDataPoint(metricDataPoints pmetric.NumberDataPointSlice, value float64, pkt *packet, stats *probing.Statistics) {
	dp := metricDataPoints.AppendEmpty()
	dp.SetDoubleValue(value)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(pkt.Timestamp))
	dp.Attributes().PutStr(ATTR_PEER_IP, pkt.Addr)
	dp.Attributes().PutStr(ATTR_PEER_NAME, stats.Addr)
}

func appendStatsDataPoint(metricDataPoints pmetric.NumberDataPointSlice, value float64, pingRes *pingResult) {
	dp := metricDataPoints.AppendEmpty()
	dp.SetDoubleValue(value)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(pingRes.StatsTimestamp))
	dp.Attributes().PutStr(ATTR_PEER_IP, pingRes.Stats.IPAddr.IP.String())
	dp.Attributes().PutStr(ATTR_PEER_NAME, pingRes.Stats.Addr)
}

func ping(target Target) (*pingResult, error) {
	pinger, err := probing.NewPinger(target.Target)
	if err != nil {
		return &pingResult{}, fmt.Errorf("failed to create pinger: %w", err)
	}

	res := &pingResult{}

	pinger.OnRecv = func(pkt *probing.Packet) {
		res.Packets = append(
			res.Packets,
			&packet{
				Timestamp: time.Now(),
				Packet:    pkt,
			},
		)
	}

	if target.PingCount != nil {
		pinger.Count = *target.PingCount
	} else {
		pinger.Count = DEFAULT_PING_COUNT
	}

	if target.PingTimeout != nil {
		pinger.Timeout = *target.PingTimeout
	} else {
		pinger.Timeout = DEFAULT_PING_TIMEOUT
	}

	err = pinger.Run()
	if err != nil {
		return &pingResult{}, fmt.Errorf("failed to run pinger: %w", err)
	}

	res.Stats = pinger.Statistics()
	res.StatsTimestamp = time.Now()

	return res, nil
}

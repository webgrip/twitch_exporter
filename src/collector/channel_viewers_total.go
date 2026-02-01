package collector

import (
	"log/slog"

	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webgrip/twitch_exporter/internal/eventsub"
)

type ChannelViewersTotalCollector struct {
	logger    *slog.Logger
	client    *helix.Client
	watchlist ChannelWatchlist

	channelViewersTotal typedDesc
}

func init() {
	// Deprecated: includes unbounded label game; use channel_core (twitch_channel_viewers).
	registerCollector("channel_viewers_total", defaultDisabled, NewChannelViewersTotalCollector)
}

func NewChannelViewersTotalCollector(logger *slog.Logger, client *helix.Client, eventsubClient *eventsub.Client, watchlist ChannelWatchlist) (Collector, error) {
	c := ChannelViewersTotalCollector{
		logger:    logger,
		client:    client,
		watchlist: watchlist,

		channelViewersTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_viewers_total"),
			"How many viewers on this live channel. If stream is offline then this is absent.",
			[]string{"username", "game"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c ChannelViewersTotalCollector) Update(ch chan<- prometheus.Metric) error {
	logins := c.watchlist.AllLogins()
	if len(logins) == 0 {
		return ErrNoData
	}

	streamsResp, err := c.client.GetStreams(&helix.StreamsParams{
		UserLogins: logins,
		First:      len(logins),
	})

	if err != nil {
		c.logger.Error("could not get streams", "err", err)
		return err
	}

	for _, s := range streamsResp.Data.Streams {
		ch <- c.channelViewersTotal.mustNewConstMetric(float64(s.ViewerCount), s.UserName, s.GameName)
	}

	return nil
}

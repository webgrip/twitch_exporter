package collector

import (
	"log/slog"

	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webgrip/twitch_exporter/internal/eventsub"
)

type channelUpCollector struct {
	logger    *slog.Logger
	client    *helix.Client
	watchlist ChannelWatchlist

	channelUp typedDesc
}

func init() {
	// Deprecated: includes unbounded label game; use channel_core (twitch_channel_live, twitch_channel_category_id).
	registerCollector("channel_up", defaultDisabled, NewChannelUpCollector)
}

func NewChannelUpCollector(logger *slog.Logger, client *helix.Client, eventsubClient *eventsub.Client, watchlist ChannelWatchlist) (Collector, error) {
	c := channelUpCollector{
		logger:    logger,
		client:    client,
		watchlist: watchlist,

		channelUp: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_up"),
			"Is the channel live.",
			[]string{"username", "game"}, nil,
		), prometheus.GaugeValue},
	}

	return c, nil
}

func (c channelUpCollector) Update(ch chan<- prometheus.Metric) error {
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

	for _, n := range logins {
		state := 0
		game := ""

		for _, s := range streamsResp.Data.Streams {
			if s.UserName == n {
				state = 1
				game = s.GameName
				break
			}
		}

		ch <- c.channelUp.mustNewConstMetric(float64(state), n, game)
	}

	return nil
}

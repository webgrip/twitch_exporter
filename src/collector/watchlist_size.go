package collector

import (
	"log/slog"

	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webgrip/twitch_exporter/internal/eventsub"
)

type watchlistSizeCollector struct {
	logger    *slog.Logger
	watchlist ChannelWatchlist

	watchlistSize typedDesc
}

func init() {
	registerCollector("watchlist", defaultEnabled, NewWatchlistSizeCollector)
}

func NewWatchlistSizeCollector(logger *slog.Logger, client *helix.Client, eventsubClient *eventsub.Client, watchlist ChannelWatchlist) (Collector, error) {
	c := watchlistSizeCollector{
		logger:    logger,
		watchlist: watchlist,
		watchlistSize: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "watchlist_size"),
			"Number of channels configured in the watchlist by role.",
			[]string{"role"},
			nil,
		), prometheus.GaugeValue},
	}
	return c, nil
}

func (c watchlistSizeCollector) Update(ch chan<- prometheus.Metric) error {
	ch <- c.watchlistSize.mustNewConstMetric(float64(c.watchlist.CountByRole(RoleSelf)), string(RoleSelf))
	ch <- c.watchlistSize.mustNewConstMetric(float64(c.watchlist.CountByRole(RoleWatch)), string(RoleWatch))
	return nil
}

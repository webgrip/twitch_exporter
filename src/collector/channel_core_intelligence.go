package collector

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webgrip/twitch_exporter/internal/eventsub"
)

type channelCoreState struct {
	live             bool
	startedAt        time.Time
	lastTitle        string
	lastCategoryID   string
	lastTransitionAt time.Time

	streamStarts    float64
	streamEnds      float64
	titleChanges    float64
	categoryChanges float64
}

type channelCoreCollector struct {
	logger    *slog.Logger
	client    *helix.Client
	watchlist ChannelWatchlist

	state map[string]*channelCoreState

	channelLive                typedDesc
	channelViewers             typedDesc
	channelStreamStartedAt     typedDesc
	channelStreamUptime        typedDesc
	channelCategoryID          typedDesc
	channelTitleChangeTotal    typedDesc
	channelCategoryChangeTotal typedDesc
	channelStreamStartsTotal   typedDesc
	channelStreamEndsTotal     typedDesc
}

func init() {
	registerCollector("channel_core", defaultEnabled, NewChannelCoreCollector)
}

func NewChannelCoreCollector(logger *slog.Logger, client *helix.Client, eventsubClient *eventsub.Client, watchlist ChannelWatchlist) (Collector, error) {
	c := &channelCoreCollector{
		logger:    logger,
		client:    client,
		watchlist: watchlist,
		state:     map[string]*channelCoreState{},
		channelLive: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_live"),
			"Whether the channel is currently live (1 = live, 0 = offline).",
			[]string{"channel", "role"}, nil,
		), prometheus.GaugeValue},
		channelViewers: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_viewers"),
			"Current viewer count for the channel (0 when offline).",
			[]string{"channel", "role"}, nil,
		), prometheus.GaugeValue},
		channelStreamStartedAt: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_stream_started_at_seconds"),
			"Unix timestamp when the current stream started (0 when offline).",
			[]string{"channel", "role"}, nil,
		), prometheus.GaugeValue},
		channelStreamUptime: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_stream_uptime_seconds"),
			"Stream uptime in seconds (0 when offline).",
			[]string{"channel", "role"}, nil,
		), prometheus.GaugeValue},
		channelCategoryID: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_category_id"),
			"Current category/game numeric ID for the channel (0 when offline/unknown).",
			[]string{"channel", "role"}, nil,
		), prometheus.GaugeValue},
		channelTitleChangeTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_title_change_total"),
			"Total number of observed title changes for the channel.",
			[]string{"channel", "role"}, nil,
		), prometheus.CounterValue},
		channelCategoryChangeTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_category_change_total"),
			"Total number of observed category/game changes for the channel.",
			[]string{"channel", "role"}, nil,
		), prometheus.CounterValue},
		channelStreamStartsTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_stream_starts_total"),
			"Total number of observed stream start transitions (offline -> live).",
			[]string{"channel", "role"}, nil,
		), prometheus.CounterValue},
		channelStreamEndsTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_stream_ends_total"),
			"Total number of observed stream end transitions (live -> offline).",
			[]string{"channel", "role"}, nil,
		), prometheus.CounterValue},
	}
	return c, nil
}

func (c *channelCoreCollector) Update(ch chan<- prometheus.Metric) error {
	logins := c.watchlist.AllLogins()
	if len(logins) == 0 {
		return ErrNoData
	}
	if c.client == nil {
		return ErrNoData
	}

	now := time.Now()

	// GetStreams only returns live streams, and limits to 100 user_logins per request.
	streamsByLogin := map[string]helix.Stream{}
	for _, batch := range chunkStrings(logins, 100) {
		resp, err := c.client.GetStreams(&helix.StreamsParams{UserLogins: batch, First: len(batch)})
		if err != nil {
			return err
		}
		for _, s := range resp.Data.Streams {
			streamsByLogin[normalizeLogin(s.UserLogin)] = s
		}
	}

	for _, login := range logins {
		login = normalizeLogin(login)
		role := c.watchlist.RoleLabelForLogin(login)
		if role == "" {
			role = string(RoleWatch)
		}

		st, ok := c.state[login]
		if !ok {
			st = &channelCoreState{}
			c.state[login] = st
		}

		s, isLive := streamsByLogin[login]
		viewers := 0.0
		startedAt := 0.0
		uptime := 0.0
		categoryID := 0.0

		if isLive {
			viewers = float64(s.ViewerCount)
			if !s.StartedAt.IsZero() {
				startedAt = float64(s.StartedAt.Unix())
				uptime = now.Sub(s.StartedAt).Seconds()
			}
			if s.GameID != "" {
				if v, err := strconv.ParseFloat(s.GameID, 64); err == nil {
					categoryID = v
				}
			}
		}

		// State machine: poller is the source of truth.
		if !st.live && isLive {
			st.streamStarts++
			st.lastTransitionAt = now
			st.startedAt = s.StartedAt
		} else if st.live && !isLive {
			st.streamEnds++
			st.lastTransitionAt = now
			st.startedAt = time.Time{}
			st.lastTitle = ""
			st.lastCategoryID = ""
		}

		if isLive {
			if st.lastTitle != "" && s.Title != "" && s.Title != st.lastTitle {
				st.titleChanges++
			}
			if st.lastCategoryID != "" && s.GameID != "" && s.GameID != st.lastCategoryID {
				st.categoryChanges++
			}
			st.lastTitle = s.Title
			st.lastCategoryID = s.GameID
		}

		st.live = isLive

		ch <- c.channelLive.mustNewConstMetric(boolToFloat(isLive), login, role)
		ch <- c.channelViewers.mustNewConstMetric(viewers, login, role)
		ch <- c.channelStreamStartedAt.mustNewConstMetric(startedAt, login, role)
		ch <- c.channelStreamUptime.mustNewConstMetric(uptime, login, role)
		ch <- c.channelCategoryID.mustNewConstMetric(categoryID, login, role)
		ch <- c.channelTitleChangeTotal.mustNewConstMetric(st.titleChanges, login, role)
		ch <- c.channelCategoryChangeTotal.mustNewConstMetric(st.categoryChanges, login, role)
		ch <- c.channelStreamStartsTotal.mustNewConstMetric(st.streamStarts, login, role)
		ch <- c.channelStreamEndsTotal.mustNewConstMetric(st.streamEnds, login, role)
	}

	return nil
}

func boolToFloat(v bool) float64 {
	if v {
		return 1
	}
	return 0
}

func chunkStrings(in []string, size int) [][]string {
	if size <= 0 {
		size = 1
	}
	if len(in) == 0 {
		return nil
	}
	out := make([][]string, 0, (len(in)+size-1)/size)
	for i := 0; i < len(in); i += size {
		end := i + size
		if end > len(in) {
			end = len(in)
		}
		out = append(out, in[i:end])
	}
	return out
}

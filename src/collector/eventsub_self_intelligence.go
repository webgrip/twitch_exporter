package collector

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webgrip/twitch_exporter/internal/eventsub"
)

// EventSub deep metrics for the self channel only.
// All labels are strictly bounded (channel, role, small enums).

type eventsubSelfCollector struct {
	logger       *slog.Logger
	client       *helix.Client
	eventsub     *eventsub.Client
	watchlist    ChannelWatchlist
	selfLogin    string
	selfUserID   string
	desiredTypes map[string]bool

	mu sync.Mutex
	st eventsubSelfState

	notificationsTotal     typedDesc
	subDesired             typedDesc
	subActive              typedDesc
	followsTotal           typedDesc
	subscriptionsTotal     typedDesc
	giftSubscriptionsTotal typedDesc
	bitsTotal              typedDesc
	bitsEventsTotal        typedDesc
	pointsRedemptionsTotal typedDesc
	raidsInTotal           typedDesc
	raidsOutTotal          typedDesc
	adsAdBreaksTotal       typedDesc
	adsMinutesTotal        typedDesc
	hypeTrainEventsTotal   typedDesc
	goalsEventsTotal       typedDesc
	pollsEventsTotal       typedDesc
	predictionsEventsTotal typedDesc
	charityEventsTotal     typedDesc
	moderationActionsTotal typedDesc
}

type eventsubSelfState struct {
	notifications map[string]float64

	follows float64

	subsByKind map[string]float64 // new|resub
	giftSubs   float64

	bitsEvents float64
	bitsTotal  float64

	pointsByGroup map[string]float64

	raidsIn  float64
	raidsOut float64

	adsBreaks  float64
	adsMinutes float64

	hypeTrainByStage   map[string]float64
	goalsByStage       map[string]float64
	pollsByStage       map[string]float64
	predictionsByStage map[string]float64
	charityByStage     map[string]float64
	moderationByAction map[string]float64
}

func init() {
	// Requires a publicly reachable webhook.
	registerCollector("eventsub_self", defaultDisabled, NewEventSubSelfCollector)
}

func NewEventSubSelfCollector(logger *slog.Logger, client *helix.Client, eventsubClient *eventsub.Client, watchlist ChannelWatchlist) (Collector, error) {
	selfLogin := watchlist.SelfLogin()
	if selfLogin == "" {
		IncCollectorDisabled("eventsub_self", "not_self_channel")
		return noopCollector{}, nil
	}
	if eventsubClient == nil {
		IncCollectorDisabled("eventsub_self", "config_disabled")
		return noopCollector{}, nil
	}
	if client == nil {
		IncCollectorDisabled("eventsub_self", "missing_token")
		return noopCollector{}, nil
	}

	users, err := client.GetUsers(&helix.UsersParams{Logins: []string{selfLogin}})
	if err != nil {
		return nil, err
	}
	if len(users.Data.Users) == 0 {
		IncCollectorDisabled("eventsub_self", "not_self_channel")
		return noopCollector{}, nil
	}

	c := &eventsubSelfCollector{
		logger:       logger,
		client:       client,
		eventsub:     eventsubClient,
		watchlist:    watchlist,
		selfLogin:    selfLogin,
		selfUserID:   users.Data.Users[0].ID,
		desiredTypes: map[string]bool{},
		st: eventsubSelfState{
			notifications: map[string]float64{},
			subsByKind:    map[string]float64{"new": 0, "resub": 0},
			pointsByGroup: map[string]float64{},

			hypeTrainByStage:   map[string]float64{"begin": 0, "progress": 0, "end": 0},
			goalsByStage:       map[string]float64{"begin": 0, "progress": 0, "end": 0},
			pollsByStage:       map[string]float64{"begin": 0, "progress": 0, "end": 0},
			predictionsByStage: map[string]float64{"begin": 0, "progress": 0, "end": 0},
			charityByStage:     map[string]float64{"begin": 0, "progress": 0, "end": 0},
			moderationByAction: map[string]float64{"timeout": 0, "ban": 0, "unban": 0, "delete": 0, "shield_on": 0, "shield_off": 0, "warn": 0, "other": 0},
		},

		notificationsTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "eventsub", "notifications_total"),
			"Total number of EventSub notifications received.",
			[]string{"channel", "role", "event_type"}, nil,
		), prometheus.CounterValue},
		subDesired: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "eventsub", "subscription_desired"),
			"Whether the exporter desires an EventSub subscription for this event type (1 = yes, 0 = no).",
			[]string{"event_type"}, nil,
		), prometheus.GaugeValue},
		subActive: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "eventsub", "subscription_active"),
			"Whether an EventSub subscription is currently active/enabled for this event type (1 = yes, 0 = no).",
			[]string{"event_type"}, nil,
		), prometheus.GaugeValue},

		followsTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_follows_total"),
			"Total number of follows for the channel (EventSub).",
			[]string{"channel"}, nil,
		), prometheus.CounterValue},
		subscriptionsTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_subscriptions_total"),
			"Total number of subscription events for the channel (EventSub).",
			[]string{"channel", "kind"}, nil,
		), prometheus.CounterValue},
		giftSubscriptionsTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_gift_subscriptions_total"),
			"Total number of gifted subscription events for the channel (EventSub).",
			[]string{"channel"}, nil,
		), prometheus.CounterValue},
		bitsTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_bits_total"),
			"Total number of bits cheered for the channel (EventSub).",
			[]string{"channel"}, nil,
		), prometheus.CounterValue},
		bitsEventsTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_bits_events_total"),
			"Total number of cheer events for the channel (EventSub).",
			[]string{"channel"}, nil,
		), prometheus.CounterValue},
		pointsRedemptionsTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_points_redemptions_total"),
			"Total number of channel points redemptions for the channel (EventSub).",
			[]string{"channel", "reward_group"}, nil,
		), prometheus.CounterValue},
		raidsInTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_raids_in_total"),
			"Total number of raids into the channel (EventSub).",
			[]string{"channel"}, nil,
		), prometheus.CounterValue},
		raidsOutTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "channel_raids_out_total"),
			"Total number of raids out of the channel (EventSub).",
			[]string{"channel"}, nil,
		), prometheus.CounterValue},

		adsAdBreaksTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "ads", "ad_breaks_total"),
			"Total number of ad breaks for the channel (EventSub), if supported by API version.",
			[]string{"channel"}, nil,
		), prometheus.CounterValue},
		adsMinutesTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "ads", "minutes_total"),
			"Total ad minutes for the channel (EventSub), if durations are provided.",
			[]string{"channel"}, nil,
		), prometheus.CounterValue},

		hypeTrainEventsTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "hype_train_events_total"),
			"Total number of hype train events for the channel (EventSub).",
			[]string{"channel", "stage"}, nil,
		), prometheus.CounterValue},
		goalsEventsTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "goals_events_total"),
			"Total number of goal events for the channel (EventSub).",
			[]string{"channel", "stage"}, nil,
		), prometheus.CounterValue},
		pollsEventsTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "polls_events_total"),
			"Total number of poll events for the channel (EventSub).",
			[]string{"channel", "stage"}, nil,
		), prometheus.CounterValue},
		predictionsEventsTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "predictions_events_total"),
			"Total number of prediction events for the channel (EventSub).",
			[]string{"channel", "stage"}, nil,
		), prometheus.CounterValue},
		charityEventsTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "charity_events_total"),
			"Total number of charity campaign events for the channel (EventSub).",
			[]string{"channel", "stage"}, nil,
		), prometheus.CounterValue},
		moderationActionsTotal: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "moderation_actions_total"),
			"Total number of moderation actions observed for the channel (EventSub).",
			[]string{"channel", "action"}, nil,
		), prometheus.CounterValue},
	}

	// Ensure at least one sample exists for reward_group series once events arrive.
	c.st.pointsByGroup[RewardGroupFor("", "")] = 0

	// Always attempt public stream online/offline (app).
	_ = c.eventsub.SubscribeApp("stream.online", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID})
	c.desire("stream.online")
	_ = c.eventsub.SubscribeApp("stream.offline", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID})
	c.desire("stream.offline")

	// Raids are public.
	_ = c.eventsub.SubscribeApp("channel.raid", "1", helix.EventSubCondition{ToBroadcasterUserID: c.selfUserID})
	c.desire("channel.raid")

	// Deep/self-only types gated by user token scopes.
	c.subscribeUserIfScope("channel.follow", "2", helix.EventSubCondition{BroadcasterUserID: c.selfUserID, ModeratorUserID: c.selfUserID}, "moderator:read:followers")
	c.subscribeUserIfScope("channel.subscribe", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "channel:read:subscriptions")
	c.subscribeUserIfScope("channel.subscription.message", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "channel:read:subscriptions")
	c.subscribeUserIfScope("channel.subscription.gift", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "channel:read:subscriptions")
	c.subscribeUserIfScope("channel.cheer", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "bits:read")
	c.subscribeUserIfScope("channel.channel_points_custom_reward_redemption.add", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "channel:read:redemptions")

	c.subscribeUserIfScope("channel.hype_train.begin", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "channel:read:hype_train")
	c.subscribeUserIfScope("channel.hype_train.progress", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "channel:read:hype_train")
	c.subscribeUserIfScope("channel.hype_train.end", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "channel:read:hype_train")

	c.subscribeUserIfScope("channel.goal.begin", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "channel:read:goals")
	c.subscribeUserIfScope("channel.goal.progress", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "channel:read:goals")
	c.subscribeUserIfScope("channel.goal.end", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "channel:read:goals")

	c.subscribeUserIfScope("channel.poll.begin", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "channel:read:polls")
	c.subscribeUserIfScope("channel.poll.progress", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "channel:read:polls")
	c.subscribeUserIfScope("channel.poll.end", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "channel:read:polls")

	c.subscribeUserIfScope("channel.prediction.begin", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "channel:read:predictions")
	c.subscribeUserIfScope("channel.prediction.progress", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "channel:read:predictions")
	c.subscribeUserIfScope("channel.prediction.lock", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "channel:read:predictions")
	c.subscribeUserIfScope("channel.prediction.end", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "channel:read:predictions")

	c.subscribeUserIfScope("channel.charity_campaign.start", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "channel:read:charity")
	c.subscribeUserIfScope("channel.charity_campaign.progress", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "channel:read:charity")
	c.subscribeUserIfScope("channel.charity_campaign.stop", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "channel:read:charity")

	c.subscribeUserIfScope("channel.ban", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "moderation:read")
	c.subscribeUserIfScope("channel.unban", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "moderation:read")
	c.subscribeUserIfScope("channel.chat.message_delete", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID, ModeratorUserID: c.selfUserID}, "moderation:read")
	c.subscribeUserIfScope("channel.shield_mode.begin", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID, ModeratorUserID: c.selfUserID}, "moderation:read")
	c.subscribeUserIfScope("channel.shield_mode.end", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID, ModeratorUserID: c.selfUserID}, "moderation:read")

	// Ads are not present in this helix version as constants; subscribe by string (best-effort).
	c.subscribeUserIfScope("channel.ad_break.begin", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "channel:read:ads")
	c.subscribeUserIfScope("channel.ad_break.end", "1", helix.EventSubCondition{BroadcasterUserID: c.selfUserID}, "channel:read:ads")

	// Handlers.
	c.on("stream.online", func(_ json.RawMessage) { c.incNotification("stream.online") })
	c.on("stream.offline", func(_ json.RawMessage) { c.incNotification("stream.offline") })

	c.on("channel.follow", func(_ json.RawMessage) {
		c.incNotification("channel.follow")
		c.mu.Lock()
		c.st.follows++
		c.mu.Unlock()
	})

	c.on("channel.subscribe", func(raw json.RawMessage) {
		c.incNotification("channel.subscribe")
		var ev struct {
			IsGift bool `json:"is_gift"`
		}
		_ = json.Unmarshal(raw, &ev)
		c.mu.Lock()
		if ev.IsGift {
			c.st.giftSubs++
		} else {
			c.st.subsByKind["new"]++
		}
		c.mu.Unlock()
	})

	c.on("channel.subscription.message", func(_ json.RawMessage) {
		c.incNotification("channel.subscription.message")
		c.mu.Lock()
		c.st.subsByKind["resub"]++
		c.mu.Unlock()
	})

	c.on("channel.subscription.gift", func(_ json.RawMessage) {
		c.incNotification("channel.subscription.gift")
		c.mu.Lock()
		c.st.giftSubs++
		c.mu.Unlock()
	})

	c.on("channel.cheer", func(raw json.RawMessage) {
		c.incNotification("channel.cheer")
		var ev struct {
			Bits int `json:"bits"`
		}
		if json.Unmarshal(raw, &ev) != nil {
			return
		}
		c.mu.Lock()
		c.st.bitsEvents++
		c.st.bitsTotal += float64(ev.Bits)
		c.mu.Unlock()
	})

	c.on("channel.channel_points_custom_reward_redemption.add", func(raw json.RawMessage) {
		c.incNotification("channel.channel_points_custom_reward_redemption.add")
		var ev struct {
			Reward struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"reward"`
		}
		if json.Unmarshal(raw, &ev) != nil {
			return
		}
		group := RewardGroupFor(ev.Reward.ID, ev.Reward.Title)
		c.mu.Lock()
		c.st.pointsByGroup[group]++
		c.mu.Unlock()
	})

	c.on("channel.raid", func(raw json.RawMessage) {
		c.incNotification("channel.raid")
		var ev struct {
			FromBroadcasterUserID string `json:"from_broadcaster_user_id"`
			ToBroadcasterUserID   string `json:"to_broadcaster_user_id"`
		}
		if json.Unmarshal(raw, &ev) != nil {
			return
		}
		c.mu.Lock()
		if ev.ToBroadcasterUserID == c.selfUserID {
			c.st.raidsIn++
		} else if ev.FromBroadcasterUserID == c.selfUserID {
			c.st.raidsOut++
		}
		c.mu.Unlock()
	})

	c.on("channel.ad_break.begin", func(raw json.RawMessage) {
		c.incNotification("channel.ad_break.begin")
		var ev struct {
			DurationSeconds int `json:"duration_seconds"`
		}
		_ = json.Unmarshal(raw, &ev)
		c.mu.Lock()
		c.st.adsBreaks++
		if ev.DurationSeconds > 0 {
			c.st.adsMinutes += float64(ev.DurationSeconds) / 60.0
		}
		c.mu.Unlock()
	})

	stageCounter := func(eventType string, stageMap *map[string]float64, stage string) func(json.RawMessage) {
		return func(_ json.RawMessage) {
			c.incNotification(eventType)
			c.mu.Lock()
			(*stageMap)[stage]++
			c.mu.Unlock()
		}
	}

	c.on("channel.hype_train.begin", stageCounter("channel.hype_train.begin", &c.st.hypeTrainByStage, "begin"))
	c.on("channel.hype_train.progress", stageCounter("channel.hype_train.progress", &c.st.hypeTrainByStage, "progress"))
	c.on("channel.hype_train.end", stageCounter("channel.hype_train.end", &c.st.hypeTrainByStage, "end"))

	c.on("channel.goal.begin", stageCounter("channel.goal.begin", &c.st.goalsByStage, "begin"))
	c.on("channel.goal.progress", stageCounter("channel.goal.progress", &c.st.goalsByStage, "progress"))
	c.on("channel.goal.end", stageCounter("channel.goal.end", &c.st.goalsByStage, "end"))

	c.on("channel.poll.begin", stageCounter("channel.poll.begin", &c.st.pollsByStage, "begin"))
	c.on("channel.poll.progress", stageCounter("channel.poll.progress", &c.st.pollsByStage, "progress"))
	c.on("channel.poll.end", stageCounter("channel.poll.end", &c.st.pollsByStage, "end"))

	c.on("channel.prediction.begin", stageCounter("channel.prediction.begin", &c.st.predictionsByStage, "begin"))
	c.on("channel.prediction.progress", stageCounter("channel.prediction.progress", &c.st.predictionsByStage, "progress"))
	c.on("channel.prediction.lock", stageCounter("channel.prediction.lock", &c.st.predictionsByStage, "progress"))
	c.on("channel.prediction.end", stageCounter("channel.prediction.end", &c.st.predictionsByStage, "end"))

	c.on("channel.charity_campaign.start", stageCounter("channel.charity_campaign.start", &c.st.charityByStage, "begin"))
	c.on("channel.charity_campaign.progress", stageCounter("channel.charity_campaign.progress", &c.st.charityByStage, "progress"))
	c.on("channel.charity_campaign.stop", stageCounter("channel.charity_campaign.stop", &c.st.charityByStage, "end"))

	c.on("channel.ban", func(raw json.RawMessage) {
		c.incNotification("channel.ban")
		var ev struct {
			IsPermanent bool `json:"is_permanent"`
		}
		_ = json.Unmarshal(raw, &ev)
		action := "ban"
		if !ev.IsPermanent {
			action = "timeout"
		}
		c.mu.Lock()
		c.st.moderationByAction[action]++
		c.mu.Unlock()
	})

	c.on("channel.unban", func(_ json.RawMessage) {
		c.incNotification("channel.unban")
		c.mu.Lock()
		c.st.moderationByAction["unban"]++
		c.mu.Unlock()
	})

	c.on("channel.chat.message_delete", func(_ json.RawMessage) {
		c.incNotification("channel.chat.message_delete")
		c.mu.Lock()
		c.st.moderationByAction["delete"]++
		c.mu.Unlock()
	})

	c.on("channel.shield_mode.begin", func(_ json.RawMessage) {
		c.incNotification("channel.shield_mode.begin")
		c.mu.Lock()
		c.st.moderationByAction["shield_on"]++
		c.mu.Unlock()
	})

	c.on("channel.shield_mode.end", func(_ json.RawMessage) {
		c.incNotification("channel.shield_mode.end")
		c.mu.Lock()
		c.st.moderationByAction["shield_off"]++
		c.mu.Unlock()
	})

	return c, nil
}

func (c *eventsubSelfCollector) on(eventType string, cb func(json.RawMessage)) {
	_ = c.eventsub.On(eventType, cb)
}

func (c *eventsubSelfCollector) desire(eventType string) {
	c.mu.Lock()
	c.desiredTypes[eventType] = true
	c.mu.Unlock()
}

func (c *eventsubSelfCollector) incNotification(eventType string) {
	c.mu.Lock()
	c.st.notifications[eventType]++
	c.mu.Unlock()
}

func (c *eventsubSelfCollector) subscribeUserIfScope(eventType string, version string, cond helix.EventSubCondition, requiredScope string) {
	if !HasUserScope(requiredScope) {
		IncCollectorDisabled("eventsub_self", "missing_scope")
		return
	}
	c.desire(eventType)
	if err := c.eventsub.SubscribeUser(eventType, version, cond); err != nil {
		c.logger.Warn("failed to subscribe to eventsub", "event_type", eventType, "err", err)
		// keep running; subscription can fail if endpoint isn't reachable yet.
	}
}

func (c *eventsubSelfCollector) Update(ch chan<- prometheus.Metric) error {
	channel := c.selfLogin
	role := string(RoleSelf)

	// Desired subscriptions.
	for eventType, desired := range c.desiredTypes {
		v := 0.0
		if desired {
			v = 1.0
		}
		ch <- c.subDesired.mustNewConstMetric(v, eventType)
	}

	// Active subscriptions (best-effort).
	active := map[string]bool{}
	if c.eventsub != nil {
		subs, err := c.eventsub.ListSubscriptions()
		if err == nil {
			for _, s := range subs {
				if s.Status == "enabled" || s.Status == "webhook_callback_verification_pending" {
					active[s.Type] = true
				}
			}
		}
	}
	for eventType := range c.desiredTypes {
		v := 0.0
		if active[eventType] {
			v = 1.0
		}
		ch <- c.subActive.mustNewConstMetric(v, eventType)
	}

	c.mu.Lock()
	st := c.st
	notifs := copyFloatMap(st.notifications)
	subs := copyFloatMap(st.subsByKind)
	points := copyFloatMap(st.pointsByGroup)
	hype := copyFloatMap(st.hypeTrainByStage)
	goals := copyFloatMap(st.goalsByStage)
	polls := copyFloatMap(st.pollsByStage)
	pred := copyFloatMap(st.predictionsByStage)
	charity := copyFloatMap(st.charityByStage)
	mod := copyFloatMap(st.moderationByAction)
	c.mu.Unlock()

	for et, v := range notifs {
		ch <- c.notificationsTotal.mustNewConstMetric(v, channel, role, et)
	}

	ch <- c.followsTotal.mustNewConstMetric(st.follows, channel)
	ch <- c.giftSubscriptionsTotal.mustNewConstMetric(st.giftSubs, channel)
	for kind, v := range subs {
		ch <- c.subscriptionsTotal.mustNewConstMetric(v, channel, kind)
	}
	ch <- c.bitsTotal.mustNewConstMetric(st.bitsTotal, channel)
	ch <- c.bitsEventsTotal.mustNewConstMetric(st.bitsEvents, channel)
	for group, v := range points {
		ch <- c.pointsRedemptionsTotal.mustNewConstMetric(v, channel, group)
	}
	ch <- c.raidsInTotal.mustNewConstMetric(st.raidsIn, channel)
	ch <- c.raidsOutTotal.mustNewConstMetric(st.raidsOut, channel)
	ch <- c.adsAdBreaksTotal.mustNewConstMetric(st.adsBreaks, channel)
	ch <- c.adsMinutesTotal.mustNewConstMetric(st.adsMinutes, channel)

	for stage, v := range hype {
		ch <- c.hypeTrainEventsTotal.mustNewConstMetric(v, channel, stage)
	}
	for stage, v := range goals {
		ch <- c.goalsEventsTotal.mustNewConstMetric(v, channel, stage)
	}
	for stage, v := range polls {
		ch <- c.pollsEventsTotal.mustNewConstMetric(v, channel, stage)
	}
	for stage, v := range pred {
		ch <- c.predictionsEventsTotal.mustNewConstMetric(v, channel, stage)
	}
	for stage, v := range charity {
		ch <- c.charityEventsTotal.mustNewConstMetric(v, channel, stage)
	}
	for action, v := range mod {
		ch <- c.moderationActionsTotal.mustNewConstMetric(v, channel, action)
	}

	return nil
}

func copyFloatMap(in map[string]float64) map[string]float64 {
	out := map[string]float64{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

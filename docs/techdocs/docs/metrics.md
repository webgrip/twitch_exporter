# Metrics

This exporter provides two classes of metrics:

- **Recommended (bounded labels)**: designed for long-term Prometheus storage
- **Legacy (high-cardinality labels)**: disabled by default, kept for compatibility

The application namespace is `twitch_`.

## Recommended (bounded labels)

### Channel state (Helix polling)

Labels are bounded to `channel` and `role`.

- `twitch_channel_live{channel,role}` (gauge 1/0)
- `twitch_channel_viewers{channel,role}` (gauge)
- `twitch_channel_stream_started_at_seconds{channel,role}` (gauge unix timestamp)
- `twitch_channel_stream_uptime_seconds{channel,role}` (gauge)
- `twitch_channel_category_id{channel,role}` (gauge)
- `twitch_channel_title_change_total{channel,role}` (counter)
- `twitch_channel_category_change_total{channel,role}` (counter)
- `twitch_channel_stream_starts_total{channel,role}` (counter)
- `twitch_channel_stream_ends_total{channel,role}` (counter)

### EventSub self-only (optional)

Disabled by default; see [EventSub](eventsub.md).

- `twitch_eventsub_notifications_total{channel,role,event_type}`
- `twitch_eventsub_subscription_desired{event_type}`
- `twitch_eventsub_subscription_active{event_type}`
- `twitch_channel_follows_total{channel}`
- `twitch_channel_subscriptions_total{channel,kind}`
- `twitch_channel_gift_subscriptions_total{channel}`
- `twitch_channel_bits_total{channel}`
- `twitch_channel_bits_events_total{channel}`
- `twitch_channel_points_redemptions_total{channel,reward_group}`
- `twitch_channel_raids_in_total{channel}`
- `twitch_channel_raids_out_total{channel}`
- `twitch_ads_ad_breaks_total{channel}`
- `twitch_ads_minutes_total{channel}`
- `twitch_hype_train_events_total{channel,stage}`
- `twitch_goals_events_total{channel,stage}`
- `twitch_polls_events_total{channel,stage}`
- `twitch_predictions_events_total{channel,stage}`
- `twitch_charity_events_total{channel,stage}`
- `twitch_moderation_actions_total{channel,action}`

### Runtime/instrumentation

- `twitch_exporter_configured` (gauge)
- `twitch_oauth_token_present{token_type="app|user"}` (gauge)
- `twitch_oauth_scope_present{scope}` (gauge; bounded to known scopes)
- `twitch_api_requests_total{api,endpoint,code_class}` (counter)
- `twitch_api_rate_limit_remaining{api}` (gauge)
- `twitch_api_rate_limit_reset_at_seconds{api}` (gauge)
- `twitch_collector_last_success_timestamp_seconds{collector}` (gauge)
- `twitch_collector_errors_total{collector,reason}` (counter)
- `twitch_collector_disabled_total{collector,reason}` (counter)
- `twitch_eventsub_signature_fail_total{reason}` (counter)

## Legacy (high-cardinality)

These are disabled by default due to unbounded label sets such as `game` and `chatter_username`.

- `twitch_channel_up{username,game}`
- `twitch_channel_viewers_total{username,game}`
- `twitch_channel_views_total{username}`
- `twitch_channel_followers_total{username}`
- `twitch_channel_subscribers_total{username,tier,gifted}`
- `twitch_channel_chat_messages_total{username,chatter_username}`

## Cardinality guidance

Prometheus label values create new time series. Avoid enabling legacy collectors unless you have a short retention period or are scraping into a separate, constrained Prometheus.

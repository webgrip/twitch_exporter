# Twitch Exporter

[![CircleCI](https://circleci.com/gh/webgrip/twitch_exporter/tree/master.svg?style=shield)][circleci]
[![Docker Pulls](https://img.shields.io/docker/pulls/webgrip/twitch-exporter.svg?maxAge=604800)][hub]
[![Go Report Card](https://goreportcard.com/badge/github.com/webgrip/twitch_exporter)][goreportcard]

Export [Twitch](https://dev.twitch.tv/docs/api/reference) metrics to [Prometheus](https://github.com/prometheus/prometheus).

To run it:

```bash
make
./twitch_exporter [flags]
```

## Exported Metrics

This exporter now provides a set of **bounded-label** metrics intended for long-term Prometheus storage.
Some legacy metrics are kept for compatibility but are **high-cardinality** and are disabled by default.

### Recommended (bounded labels)

**Core Helix polling (enabled by default):**

| Metric | Meaning | Labels |
| ------ | ------- | ------ |
| twitch_channel_live | Whether the channel is currently live (1/0). | channel, role |
| twitch_channel_viewers | Current viewer count (0 when offline). | channel, role |
| twitch_channel_stream_started_at_seconds | Stream start time as unix timestamp (0 when offline). | channel, role |
| twitch_channel_stream_uptime_seconds | Stream uptime seconds (0 when offline). | channel, role |
| twitch_channel_category_id | Current category/game numeric id (0 when offline/unknown). | channel, role |
| twitch_channel_title_change_total | Observed title changes (poll-based). | channel, role |
| twitch_channel_category_change_total | Observed category changes (poll-based). | channel, role |
| twitch_channel_stream_starts_total | Observed stream starts (offline→live). | channel, role |
| twitch_channel_stream_ends_total | Observed stream ends (live→offline). | channel, role |

**EventSub self-only (disabled by default):**

| Metric | Meaning | Labels |
| ------ | ------- | ------ |
| twitch_eventsub_notifications_total | Total EventSub notifications received. | channel, role, event_type |
| twitch_eventsub_subscription_desired | Exporter desires a subscription for this type (1/0). | event_type |
| twitch_eventsub_subscription_active | Subscription appears active/enabled (best-effort) (1/0). | event_type |
| twitch_channel_follows_total | Follows observed via EventSub. | channel |
| twitch_channel_subscriptions_total | Subscriptions observed via EventSub. | channel, kind |
| twitch_channel_gift_subscriptions_total | Gift subscription events observed via EventSub. | channel |
| twitch_channel_bits_total | Bits observed via EventSub. | channel |
| twitch_channel_bits_events_total | Cheer events observed via EventSub. | channel |
| twitch_channel_points_redemptions_total | Channel points redemptions observed via EventSub. | channel, reward_group |
| twitch_channel_raids_in_total | Raids into the channel observed via EventSub. | channel |
| twitch_channel_raids_out_total | Raids out of the channel observed via EventSub. | channel |
| twitch_ads_ad_breaks_total | Ad breaks observed via EventSub (if supported). | channel |
| twitch_ads_minutes_total | Ad minutes observed via EventSub (if durations provided). | channel |
| twitch_hype_train_events_total | Hype train lifecycle events observed via EventSub. | channel, stage |
| twitch_goals_events_total | Goal lifecycle events observed via EventSub. | channel, stage |
| twitch_polls_events_total | Poll lifecycle events observed via EventSub. | channel, stage |
| twitch_predictions_events_total | Prediction lifecycle events observed via EventSub. | channel, stage |
| twitch_charity_events_total | Charity lifecycle events observed via EventSub. | channel, stage |
| twitch_moderation_actions_total | Moderation actions observed via EventSub. | channel, action |

**Runtime/instrumentation (enabled automatically):**

| Metric | Meaning | Labels |
| ------ | ------- | ------ |
| twitch_oauth_token_present | Whether an OAuth token is present (1/0). | token_type |
| twitch_oauth_scope_present | Whether a known OAuth scope is present on the validated user token (1/0). | scope |
| twitch_api_requests_total | Total Twitch API HTTP requests by surface/endpoint/status class. | api, endpoint, code_class |
| twitch_api_rate_limit_remaining | Rate limit remaining if provided. | api |
| twitch_api_rate_limit_reset_at_seconds | Rate limit reset timestamp if provided. | api |
| twitch_collector_last_success_timestamp_seconds | Last successful collector run time. | collector |
| twitch_collector_errors_total | Collector errors by reason. | collector, reason |
| twitch_collector_disabled_total | Collector disabled count by reason. | collector, reason |
| twitch_eventsub_signature_fail_total | EventSub webhook signature failures. | reason |

### Legacy (high-cardinality labels)

These metrics are retained for compatibility but are **not recommended** for long-term storage because they include
labels like `game` and `chatter_username`.

| Metric | Meaning | Labels |
| ------ | ------- | ------ |
| twitch_channel_up | Is the twitch channel Online. | username, game |
| twitch_channel_viewers_total | Is the total number of viewers on an online twitch channel. | username, game |
| twitch_channel_views_total | Is the total number of views on a twitch channel. | username |
| twitch_channel_followers_total | Is the total number of follower on a twitch channel. | username |
| twitch_channel_subscribers_total | Is the total number of subscriber on a twitch channel. | username, tier, gifted |
| twitch_channel_chat_messages_total | Is the total number of chat messages from a user within a channel. | username, chatter_username |

## Self-only mode (your channel)

If you only want metrics for your own channel, set `TWITCH_CHANNEL_1=yonnurs` in `.env` and run `docker compose up --build`.
The exporter treats the first `--twitch.channel` as `role=self` automatically (backwards-compatible).

### Flags

```bash
./twitch_exporter --help
```

* __`twitch.self-channel`:__ Your own Twitch channel login (role=self). Required for privileged/self-only metrics.
* __`twitch.watch-channel`:__ A Twitch channel login to watch (role=watch). Can be provided multiple times; max 100.
* __`twitch.channel`:__ (Deprecated) Name of a Twitch channel. Treated as role=watch. For backwards-compat, the first `--twitch.channel` is treated as `role=self` if `--twitch.self-channel` is not provided.
* __`twitch.client-id`:__ The client ID to request the New Twitch API (helix).
* __`twitch.access-token`:__ The access token to request the New Twitch API (helix).
* __`log.format`:__ Set the log target and format. Example: `logger:syslog?appname=bob&local=7`
    or `logger:stdout?json=true`
* __`log.level`:__ Logging level. `info` by default.
* __`version`:__ Show application version.
* __`web.listen-address`:__ Address to listen on for web interface and telemetry.
* __`web.telemetry-path`:__ Path under which to expose metrics.
* __`eventsub.enabled`:__ Enable eventsub endpoint (default: false).
* __`eventsub.webhook-url`:__ The url your collector will be expected to be hosted at, eg: http://example.svc/eventsub (Must end with `/eventsub`).
* __`eventsub.webhook-secret`:__ Secure 1-100 character secret for your eventsub validation
* __`--[no-]collector.channel_core`:__ Enable the channel_core collector (default: enabled).
* __`--[no-]collector.watchlist`:__ Enable the watchlist collector (default: enabled).
* __`--[no-]collector.eventsub_self`:__ Enable the eventsub_self collector (default: disabled**).
* __`--[no-]collector.channel_followers_total`:__ Enable the channel_followers_total collector (default: enabled).
* __`--[no-]collector.channel_subscribers_total`:__ Enable the channel_subscribers_total collector (default: disabled*).
* __`--[no-]collector.channel_up`:__ Enable the channel_up collector (default: disabled***).
* __`--[no-]collector.channel_viewers_total`:__ Enable the channel_viewers_total collector (default: disabled***).
* __`--[no-]collector.channel_chat_messages_total`:__ Enable the channel_chat_messages_total (default: disabled**).

```
* Disabled due to the requirement of a user access token, which must be acquired outside of the exporter
** Disabled due to requiring an EventSub webhook endpoint
*** Disabled by default due to high-cardinality labels (see "Legacy" metrics above)
```

## Event-sub

Event-sub metrics are disabled by default due to requiring a public endpoint to be exposed and more permissions and setup.
Due to the likeliness that you do not want to expose the service publicly and go through too much effort, it is disabled by
default.

If you wish to use eventsub based metrics then you should deploy an instance of the exporter just for the user that needs
the eventsub metrics, such as your own channel, and just collect the privileged metrics using that exporter.

> Todo: Instead of disabling all other collectors, functionality to set collectors should be implemented

### Setting up eventsub metrics

You can read more about the process [here](https://dev.twitch.tv/docs/chat/authenticating/)

1. Install the twitch-cli
1. Ensure your twitch app has localhost:3000 added as a redirect uri
1. Create a user token with the scopes you want to observe. For `eventsub_self`, the exporter understands these scopes:
  `bits:read`, `channel:read:subscriptions`, `channel:read:redemptions`, `channel:read:ads`, `channel:read:charity`,
  `channel:read:goals`, `channel:read:hype_train`, `channel:read:polls`, `channel:read:predictions`,
  `moderator:read:followers`, `moderation:read`.
1. Start the exporter with `client-id`, `client-secret`, `access-token`, and `refresh-token` defined, plus EventSub webhook settings.

```
cd src && go run . \
  --twitch.client-id xxx \
  --twitch.client-secret xxx \
  --twitch.access-token xxx \
  --twitch.refresh-token xxx \
  --twitch.self-channel surdaft \
  --eventsub.enabled \
  --eventsub.webhook-url 'https://xxx/eventsub' \
  --eventsub.webhook-secret xxxx \
  --collector.eventsub_self \
  --collector.channel_chat_messages_total \
  --collector.channel_subscribers_total \
  --no-collector.channel_followers_total \
  --no-collector.channel_up \
  --no-collector.channel_viewers_total
```

## Useful Queries

TODO

## Using Docker

You can deploy this exporter using the [webgrip/twitch-exporter](https://hub.docker.com/r/webgrip/twitch-exporter/) Docker image.

For example:

```bash
docker pull webgrip/twitch-exporter

docker run -d -p 9184:9184 \
        webgrip/twitch-exporter \
        --twitch.client-id <secret> \
        --twitch.access-token <secret> \
        --twitch.channel dam0un
```

## Using Docker Compose

For local development and running the exporter without installing Go locally:

```bash
cp .env.example .env
# edit .env

docker compose up --build
```

If you see `twitch_exporter_configured 0` in `/metrics`, set `TWITCH_CLIENT_ID` and `TWITCH_CLIENT_SECRET` in `.env`.

## Using Helm

This repository no longer ships a Helm chart.

If you want to deploy to Kubernetes, use a generic chart like `oci://ghcr.io/bjw-s-labs/helm/app-template` and configure:

- Container args/env to match the flags in this README
- Service on port `9184`
- Ingress so `GET /metrics` is reachable by Prometheus
- If enabling EventSub: a publicly reachable HTTPS `POST /eventsub` endpoint, with request body and Twitch headers preserved

[circleci]: https://circleci.com/gh/webgrip/twitch_exporter
[hub]: https://hub.docker.com/r/webgrip/twitch-exporter/
[goreportcard]: https://goreportcard.com/report/github.com/webgrip/twitch_exporter

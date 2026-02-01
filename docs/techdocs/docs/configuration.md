# Configuration

The exporter is configured primarily via flags (Kingpin). For container usage, you typically map environment variables into those flags.

## Required

### Helix app credentials

- `--twitch.client-id` (required)
- `--twitch.client-secret` (recommended)

If `--twitch.client-secret` is missing, the exporter will still start, but it disables default collectors and exports `twitch_exporter_configured 0`.

## Channels

### Preferred flags

- `--twitch.self-channel=<login>`
- `--twitch.watch-channel=<login>` (repeatable, max 100)

### Legacy flag

- `--twitch.channel=<login>` (deprecated)

Back-compat behavior:

- If `--twitch.self-channel` is not set and you pass one or more `--twitch.channel`, the **first** legacy channel is treated as `role=self`.

## Web server

- `--web.listen-address` (default provided by exporter-toolkit flags)
- `--web.telemetry-path` (default `/metrics`)

The exporter uses Prometheus exporter-toolkit, so you can also use the toolkit’s `--web.config.file` to enable TLS and/or basic auth.

## EventSub (optional)

EventSub is off by default.

- `--eventsub.enabled`
- `--eventsub.webhook-url=https://<public-host>/eventsub` (must end with `/eventsub`)
- `--eventsub.webhook-secret=<secret>`

Important Twitch constraints:

- Webhook callback must be reachable via **HTTPS on port 443** for real subscriptions.
- Secret should be **ASCII 10–100 chars** (Twitch requirement).

## Reward grouping (bounded labels)

To keep `reward_group` bounded for channel point redemptions:

- `--twitch.reward-group.default=default`
- `--twitch.reward-group.unknown=other`
- `--twitch.reward-group.max=20`
- `--twitch.reward-group.id=<reward_id>:<group>` (repeatable)
- `--twitch.reward-group.title=<reward_title>:<group>` (repeatable; title is normalized to lowercase)

If the number of unique groups exceeds `--twitch.reward-group.max`, the exporter exits with an error.

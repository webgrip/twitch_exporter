# Project overview

This exporter exposes Prometheus metrics under a configurable HTTP path (default: `/metrics`).

## Data sources

### Helix polling (default)

Uses the Twitch Helix API to poll channel state periodically during scrapes.

- Designed to keep label cardinality **bounded** (e.g., `channel`, `role`)
- Suitable for long-term retention and remote_write

### EventSub webhook (optional)

Optionally exposes an `/eventsub` webhook handler and subscribes to EventSub topics.

- Intended for **self channel only** (privileged metrics)
- Requires a publicly reachable HTTPS endpoint (Twitch requirement)
- Requires an **app access token** for webhook subscriptions; some topics also require your app to have user-granted scopes

## Watchlist roles

Channels are tracked with a `role` label:

- `role=self` — your own channel (required for EventSub/self-only collectors)
- `role=watch` — channels you’re monitoring in a bounded watch list

The watch list is bounded to **max 100** watch channels.

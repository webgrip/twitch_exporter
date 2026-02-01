# Runbook: collectors disabled

## Symptoms

- Missing expected metrics
- `twitch_scrape_collector_success{collector="..."} == 0`
- `twitch_collector_disabled_total{collector="..."} > 0`

## Checks

1. Confirm required credentials:
   - `twitch_exporter_configured == 1`

2. Confirm watchlist role expectations:
   - Self-only collectors require a configured `role=self`

3. If EventSub collector:
   - `--eventsub.enabled` is set
   - webhook URL ends with `/eventsub`
   - webhook secret matches and is within Twitch constraints (ASCII 10–100 chars)

4. If scope-gated:
   - check `twitch_oauth_scope_present{scope="..."} == 1`

## Actions

- Add required token/scopes, restart exporter
- For EventSub, validate webhook by using Twitch CLI challenge verification

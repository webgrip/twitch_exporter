# Runbook: EventSub verification failures

## Symptoms

- 403 responses on `/eventsub`
- `twitch_eventsub_signature_fail_total{reason="bad_signature"}` increasing

## Common causes

- Webhook secret mismatch between exporter and Twitch subscription
- Reverse proxy modifies request body (compression, JSON parsing/re-encoding, etc.)
- Missing required Twitch headers forwarded to the exporter

## Actions

1. Confirm proxy forwards the raw request body unmodified.
2. Confirm proxy preserves these headers:
   - `Twitch-Eventsub-Message-Id`
   - `Twitch-Eventsub-Message-Timestamp`
   - `Twitch-Eventsub-Message-Signature`
3. Verify webhook secret matches what was used when creating subscriptions.

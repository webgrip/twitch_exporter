# EventSub

EventSub support is **disabled by default** because it requires exposing a webhook endpoint.

## What it does

When enabled, the exporter:

- Serves an HTTP handler at `POST /eventsub`
- Verifies the Twitch signature using HMAC-SHA256
- Registers EventSub subscriptions (webhook transport)
- Updates self-only metrics as notifications arrive

## Requirements

### Public webhook

For real Twitch subscriptions, your callback must be:

- HTTPS
- Port 443
- Publicly reachable

For local testing, the Twitch CLI can send verification/notification events without requiring public HTTPS.

### Secrets

Twitch requires a webhook secret that is:

- ASCII
- 10–100 characters

The exporter uses the webhook secret to verify `Twitch-Eventsub-Message-Signature`.

## Enabling

Minimum flags:

- `--eventsub.enabled`
- `--eventsub.webhook-url=https://<public-host>/eventsub`
- `--eventsub.webhook-secret=<secret>`
- `--collector.eventsub_self`
- `--twitch.self-channel=<login>`

For privileged topics, you also need a user token + refresh token and the appropriate scopes.

## Signature verification

Twitch signs each message with:

- `message = <msg_id> + <timestamp> + <raw_body>`
- `hmac = HMAC_SHA256(secret, message)`
- header: `Twitch-Eventsub-Message-Signature: sha256=<hex>`

If verification fails, the exporter returns `403` and increments `twitch_eventsub_signature_fail_total{reason=...}`.

## Testing

Using Twitch CLI (recommended for local):

- Send a verification challenge
- Trigger a sample event

See Twitch docs for the exact CLI commands and supported triggers.

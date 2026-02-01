# Troubleshooting

## Exporter starts but shows no Twitch data

Check:

- `twitch_exporter_configured` is `1`
- Logs for “collectors are disabled”

Common causes:

- Missing `--twitch.client-secret` (or missing env var backing it)
- Invalid credentials or revoked tokens

## Some collectors disabled

Check:

- `twitch_collector_disabled_total{collector,reason}`

Common reasons:

- `missing_token` / `missing_scope` for privileged collectors
- EventSub collectors require `--eventsub.enabled` and webhook configuration

## EventSub webhook verification fails

Check:

- `twitch_eventsub_signature_fail_total{reason}`
- Ensure your reverse proxy does not modify the request body
- Ensure you are using the same webhook secret you used for subscription creation

## Rate limiting

Check:

- `twitch_api_requests_total{api,endpoint,code_class}`
- `twitch_api_rate_limit_remaining{api}`
- `twitch_api_rate_limit_reset_at_seconds{api}`

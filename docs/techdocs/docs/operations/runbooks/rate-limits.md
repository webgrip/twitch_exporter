# Runbook: Twitch API rate limits

## Symptoms

- Scrapes failing intermittently
- Increased `twitch_api_requests_total{code_class="429"}`

## Checks

- `twitch_api_rate_limit_remaining{api="helix"}`
- Exporter logs around collector failures

## Actions

- Reduce scrape frequency
- Reduce enabled collectors
- Reduce watched channels (max 100 supported for `role=watch`)
- Consider running separate exporters for different use cases (e.g., one per “self” channel)

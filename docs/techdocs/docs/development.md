# Development

## Repo layout

- `src/` contains the Go module
- `src/collector/` contains collectors and metric definitions
- `src/internal/eventsub/` contains EventSub webhook plumbing

## Common tasks

- Build: `make`
- Format: `make common-format`
- Lint: `make common-lint`
- Test: `make common-test`

## Adding a collector

Collectors are registered via `registerCollector` with a default enabled/disabled state.

Guidelines:

- Keep labels bounded
- Prefer `role` and stable identifiers over names that change frequently
- If an API surface requires user scopes, export a clear disabled reason via `twitch_collector_disabled_total{...}`

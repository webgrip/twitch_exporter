# Quickstart

## Local (Docker Compose)

1. Copy example env file:

   - `cp .env.example .env`

2. Edit `.env` and set at least:

   - `TWITCH_CLIENT_ID`
   - `TWITCH_CLIENT_SECRET`

3. Start:

   - `docker compose up --build`

4. Verify:

   - Open `http://localhost:9184/metrics`
   - Ensure `twitch_exporter_configured` is `1`

## Local (Go)

1. Build:

   - `make`

2. Run:

   - `./twitch_exporter --help`

3. Start with app credentials and a channel:

   - `./twitch_exporter --twitch.client-id ... --twitch.client-secret ... --twitch.self-channel <login>`

## Next

- See [Configuration](configuration.md)
- See [Metrics](metrics.md)

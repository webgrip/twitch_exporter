# Deployment: Docker

## Using the published image

The upstream README documents `webgrip/twitch-exporter` on Docker Hub.

Typical options:

- Pass flags as container args
- Provide secrets via environment variables or secret stores
- Expose port 9184

## Using Docker Compose (this repo)

This repo ships a `docker-compose.yml` for local development.

1. Copy `.env.example` to `.env`
2. Set `TWITCH_CLIENT_ID` and `TWITCH_CLIENT_SECRET`
3. Run `docker compose up --build`

## Hardening

For production, strongly consider:

- Enabling TLS and/or basic auth via `--web.config.file` (exporter-toolkit)
- Restricting ingress to Prometheus only
- Running a separate instance for EventSub self-only metrics (so only that instance is publicly exposed)

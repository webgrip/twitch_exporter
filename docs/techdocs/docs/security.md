# Security

## Secrets handling

Treat these values as secrets:

- Twitch client secret
- User access token
- User refresh token
- EventSub webhook secret

Do not log them, do not bake them into images.

## Endpoint exposure

Expose `/metrics` only to Prometheus (or an internal network).

If you enable EventSub:

- You must expose `POST /eventsub` publicly
- Prefer running a dedicated exporter instance for EventSub and keep the “watch” exporters private

## TLS / authentication

The exporter uses Prometheus exporter-toolkit. Use `--web.config.file` to enable:

- TLS
- Basic auth

## Supply chain

The container image builds a static Go binary and runs it in a distroless nonroot base image.

# Deployment: Kubernetes

This repository does not ship a Helm chart.

Use a generic chart (or plain manifests) and configure:

- Container args to match flags in [Configuration](../configuration.md)
- Service port `9184`
- Prometheus scrape config for `/metrics`

## EventSub in Kubernetes

If enabling EventSub:

- Ingress must expose `POST /eventsub` publicly over HTTPS
- Ensure the ingress/controller preserves Twitch headers
- Avoid body transformations (signature verification requires raw body)

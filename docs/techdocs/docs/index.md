---
title: "Twitch Exporter"
description: "Prometheus exporter for Twitch Helix + EventSub metrics"
tags:
  - twitch
  - prometheus
  - exporter
hide:
  - toc
search:
  boost: 5
---

# Twitch Exporter

Prometheus exporter that collects Twitch channel metrics via:

- **Helix polling** (bounded-label metrics suitable for long-term storage)
- **EventSub webhook** (self-only, privileged metrics; requires a public webhook)

## Where to start

- New here? Read [Quickstart](quickstart.md)
- Deploying? Read [Docker deployment](deployment/docker.md)
- Looking for what to scrape? Read [Metrics](metrics.md)
- Enabling deep self metrics? Read [EventSub](eventsub.md)

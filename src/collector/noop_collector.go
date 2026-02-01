package collector

import "github.com/prometheus/client_golang/prometheus"

type noopCollector struct{}

func (noopCollector) Update(ch chan<- prometheus.Metric) error { return nil }

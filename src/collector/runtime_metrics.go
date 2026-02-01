package collector

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	runtimeMetricsOnce sync.Once
	metrics            *runtimeMetrics
)

type runtimeMetrics struct {
	collectorLastSuccess *prometheus.GaugeVec
	collectorErrorsTotal *prometheus.CounterVec
	collectorDisabled    *prometheus.CounterVec

	oauthTokenPresent *prometheus.GaugeVec
	oauthScopePresent *prometheus.GaugeVec

	apiRequestsTotal      *prometheus.CounterVec
	apiRateLimitRemaining *prometheus.GaugeVec
	apiRateLimitResetAt   *prometheus.GaugeVec

	eventsubSignatureFail *prometheus.CounterVec
}

func getRuntimeMetrics() *runtimeMetrics {
	runtimeMetricsOnce.Do(func() {
		metrics = &runtimeMetrics{
			collectorLastSuccess: prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: prometheus.BuildFQName(namespace, "collector", "last_success_timestamp_seconds"),
				Help: "Unix timestamp of the last successful collector run.",
			}, []string{"collector"}),
			collectorErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: prometheus.BuildFQName(namespace, "collector", "errors_total"),
				Help: "Total number of collector errors by reason.",
			}, []string{"collector", "reason"}),
			collectorDisabled: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: prometheus.BuildFQName(namespace, "collector", "disabled_total"),
				Help: "Total number of times a collector was disabled due to missing capabilities or config.",
			}, []string{"collector", "reason"}),

			oauthTokenPresent: prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: prometheus.BuildFQName(namespace, "oauth", "token_present"),
				Help: "Whether an OAuth token is present (1 = yes, 0 = no).",
			}, []string{"token_type"}),
			oauthScopePresent: prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: prometheus.BuildFQName(namespace, "oauth", "scope_present"),
				Help: "Whether a known OAuth scope is present on the validated user token (1 = yes, 0 = no).",
			}, []string{"scope"}),

			apiRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: prometheus.BuildFQName(namespace, "api", "requests_total"),
				Help: "Total Twitch API HTTP requests by API surface, endpoint, and status class.",
			}, []string{"api", "endpoint", "code_class"}),
			apiRateLimitRemaining: prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: prometheus.BuildFQName(namespace, "api", "rate_limit_remaining"),
				Help: "Twitch API rate limit remaining, if provided by response headers.",
			}, []string{"api"}),
			apiRateLimitResetAt: prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: prometheus.BuildFQName(namespace, "api", "rate_limit_reset_at_seconds"),
				Help: "Unix timestamp when the Twitch API rate limit resets, if provided by response headers.",
			}, []string{"api"}),
			eventsubSignatureFail: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: prometheus.BuildFQName(namespace, "eventsub", "signature_fail_total"),
				Help: "Total number of EventSub webhook signature verification failures.",
			}, []string{"reason"}),
		}
	})
	return metrics
}

// runtimeCollectors returns all internal metric vectors that should be exposed.
func runtimeCollectors() []prometheus.Collector {
	m := getRuntimeMetrics()
	return []prometheus.Collector{
		m.collectorLastSuccess,
		m.collectorErrorsTotal,
		m.collectorDisabled,
		m.oauthTokenPresent,
		m.oauthScopePresent,
		m.apiRequestsTotal,
		m.apiRateLimitRemaining,
		m.apiRateLimitResetAt,
		m.eventsubSignatureFail,
	}
}

func SetOAuthTokenPresent(tokenType string, present bool) {
	v := 0.0
	if present {
		v = 1.0
	}
	getRuntimeMetrics().oauthTokenPresent.WithLabelValues(tokenType).Set(v)
}

// SetKnownOAuthScopes sets the known scope gauges to 0/1. Scopes not in knownScopes are ignored.
func SetKnownOAuthScopes(knownScopes []string, presentScopes []string) {
	m := getRuntimeMetrics()
	present := map[string]struct{}{}
	for _, s := range presentScopes {
		present[strings.TrimSpace(s)] = struct{}{}
	}
	for _, s := range knownScopes {
		_, ok := present[s]
		if ok {
			m.oauthScopePresent.WithLabelValues(s).Set(1)
		} else {
			m.oauthScopePresent.WithLabelValues(s).Set(0)
		}
	}
}

func IncCollectorDisabled(collectorName string, reason string) {
	getRuntimeMetrics().collectorDisabled.WithLabelValues(collectorName, reason).Inc()
}

func ObserveCollectorSuccess(collectorName string, at time.Time) {
	getRuntimeMetrics().collectorLastSuccess.WithLabelValues(collectorName).Set(float64(at.Unix()))
}

func ObserveCollectorError(collectorName string, reason string) {
	getRuntimeMetrics().collectorErrorsTotal.WithLabelValues(collectorName, reason).Inc()
}

func ObserveAPIResponse(api string, endpoint string, statusCode int, headers http.Header) {
	m := getRuntimeMetrics()
	codeClass := "other"
	switch {
	case statusCode >= 200 && statusCode < 300:
		codeClass = "2xx"
	case statusCode >= 400 && statusCode < 500:
		codeClass = "4xx"
	case statusCode >= 500 && statusCode < 600:
		codeClass = "5xx"
	}
	m.apiRequestsTotal.WithLabelValues(api, endpoint, codeClass).Inc()

	// These headers are present on Helix responses.
	remaining := headerAny(headers, "Ratelimit-Remaining", "RateLimit-Remaining")
	reset := headerAny(headers, "Ratelimit-Reset", "RateLimit-Reset")
	if remaining != "" {
		if v, err := strconv.ParseFloat(remaining, 64); err == nil {
			m.apiRateLimitRemaining.WithLabelValues(api).Set(v)
		}
	}
	if reset != "" {
		if v, err := strconv.ParseFloat(reset, 64); err == nil {
			m.apiRateLimitResetAt.WithLabelValues(api).Set(v)
		}
	}
}

func IncEventSubSignatureFail(reason string) {
	if reason == "" {
		reason = "other"
	}
	getRuntimeMetrics().eventsubSignatureFail.WithLabelValues(reason).Inc()
}

func ClassifyErrorReason(err error) string {
	if err == nil {
		return "other"
	}
	if ne, ok := err.(net.Error); ok {
		if ne.Timeout() {
			return "timeout"
		}
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "rate limit") || strings.Contains(msg, "ratelimit") || strings.Contains(msg, "429"):
		return "rate_limited"
	case strings.Contains(msg, "401") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "invalid oauth"):
		return "auth"
	case strings.Contains(msg, "status 4") || strings.Contains(msg, " 4"):
		return "http_4xx"
	case strings.Contains(msg, "status 5") || strings.Contains(msg, " 5"):
		return "http_5xx"
	case strings.Contains(msg, "json") || strings.Contains(msg, "decode") || strings.Contains(msg, "unmarshal"):
		return "decode"
	default:
		return "other"
	}
}

func headerAny(h http.Header, keys ...string) string {
	for _, k := range keys {
		v := h.Get(k)
		if v != "" {
			return v
		}
	}
	return ""
}

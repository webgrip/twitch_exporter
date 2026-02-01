package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	kingpin "github.com/alecthomas/kingpin/v2"
	"github.com/nicklaw5/helix/v2"
	"github.com/prometheus/client_golang/prometheus"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/promslog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
	"github.com/webgrip/twitch_exporter/collector"
	"github.com/webgrip/twitch_exporter/internal/eventsub"
)

var (
	metricsPath = kingpin.Flag("web.telemetry-path",
		"Path under which to expose metrics.").
		Default("/metrics").String()

	// twitch app access token config
	twitchClientID = kingpin.Flag("twitch.client-id",
		"Client ID for the Twitch Helix API.").Required().String()
	twitchClientSecret = kingpin.Flag("twitch.client-secret",
		"Client Secret for the Twitch Helix API.").String()

	// twitch client access token config
	twitchAccessToken = kingpin.Flag("twitch.access-token",
		"Access Token for the Twitch Helix API.").String()
	twitchRefreshToken = kingpin.Flag("twitch.refresh-token",
		"Refresh Token for the Twitch Helix API.").String()
	eventSubEnabled = kingpin.Flag("eventsub.enabled",
		"Enable the Twitch Eventsub API.").Default("false").Bool()
	eventSubWebhookURL = kingpin.Flag("eventsub.webhook-url",
		"The url your collector will be expected to be hosted at, eg: http://example.svc/eventsub (Must end with `/eventsub`).").Default("").String()
	eventSubWebhookSecret = kingpin.Flag("eventsub.webhook-secret",
		"Secure 1-100 character secret for your eventsub validation.").Default("").String()

	// collector configs
	// the twitch channel is a global config for all collectors, and is
	// defined at the root level. Individual collectors may have their own
	// configurations, which are defined within the collector itself.
	twitchChannel = Channels(kingpin.Flag("twitch.channel",
		"(Deprecated) Name of a Twitch Channel to request metrics for. Treated as role=watch."))

	// watchlist roles
	twitchSelfChannel = kingpin.Flag("twitch.self-channel",
		"Your own Twitch channel login (role=self). Required for privileged/self-only metrics.").Default("").String()
	twitchWatchChannels = Channels(kingpin.Flag("twitch.watch-channel",
		"A Twitch channel login to watch (role=watch). Can be provided multiple times; max 100."))

	// reward grouping for channel points redemptions
	rewardGroupDefault = kingpin.Flag("twitch.reward-group.default",
		"Default reward group label for points redemptions when reward id/title is empty.").Default("default").String()
	rewardGroupUnknown = kingpin.Flag("twitch.reward-group.unknown",
		"Reward group label used when a reward is not mapped.").Default("other").String()
	rewardGroupMax = kingpin.Flag("twitch.reward-group.max",
		"Maximum number of unique reward_group label values allowed.").Default("20").Int()

	rewardGroupByID = KeyValueMap(kingpin.Flag("twitch.reward-group.id",
		"Map a channel points reward id to a reward_group label (repeatable). Format: <reward_id>:<group>."))
	rewardGroupByTitle = KeyValueMap(kingpin.Flag("twitch.reward-group.title",
		"Map a channel points reward title to a reward_group label (repeatable). Format: <reward_title>:<group>."))
)

type keyValueMap map[string]string

func KeyValueMap(s kingpin.Settings) *keyValueMap {
	target := &keyValueMap{}
	s.SetValue(target)
	return target
}

func (m keyValueMap) IsCumulative() bool { return true }

func (m *keyValueMap) Set(v string) error {
	if *m == nil {
		*m = keyValueMap{}
	}
	parts := strings.SplitN(v, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("expected <key>:<value>, got %q", v)
	}
	key := strings.TrimSpace(parts[0])
	val := strings.TrimSpace(parts[1])
	if key == "" || val == "" {
		return fmt.Errorf("expected non-empty <key>:<value>, got %q", v)
	}
	(*m)[key] = val
	return nil
}

func (m keyValueMap) String() string {
	return fmt.Sprintf("%v", map[string]string(m))
}

type promHTTPLogger struct {
	logger *slog.Logger
}

func (l promHTTPLogger) Println(v ...interface{}) {
	l.logger.Error(fmt.Sprint(v...))
}

// Channels creates a collection of Channels from a kingpin command line argument.
func Channels(s kingpin.Settings) (target *collector.ChannelNames) {
	target = &collector.ChannelNames{}
	s.SetValue(target)
	return target
}

func init() {
	prometheus.MustRegister(versioncollector.NewCollector("twitch_exporter"))
}

func main() {
	promslogConfig := &promslog.Config{}
	flag.AddFlags(kingpin.CommandLine, promslogConfig)

	var webConfig = webflag.AddFlags(kingpin.CommandLine, "0.0.0.0:9184")
	kingpin.Version(version.Print("twitch_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	logger := promslog.New(promslogConfig)
	logger.Info("Starting twitch_exporter", "version", version.Info())
	logger.Info("", "build_context", version.BuildContext())

	configured := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "twitch_exporter_configured",
		Help: "Whether the exporter has the minimum Twitch credentials configured (1 = yes, 0 = no).",
	})

	var client *helix.Client
	var err error

	clientType := "app"

	if *twitchClientID == "" || *twitchClientSecret == "" {
		logger.Warn("Twitch credentials not configured; exporter will start but collectors are disabled", "missing_client_id", *twitchClientID == "", "missing_client_secret", *twitchClientSecret == "")
		collector.DisableDefaultCollectors()
		configured.Set(0)
	} else {
		configured.Set(1)
	}

	if *twitchAccessToken != "" && *twitchRefreshToken != "" {
		clientType = "user"
	}

	appTokenPresent := *twitchClientID != "" && *twitchClientSecret != ""
	userTokenPresent := *twitchAccessToken != "" && *twitchRefreshToken != ""

	// Capabilities surface: token presence.
	collector.SetOAuthTokenPresent("app", appTokenPresent)
	collector.SetOAuthTokenPresent("user", userTokenPresent)

	// Validate user token scopes (if present) so missing scopes are obvious in metrics.
	validatedScopes := []string{}
	if userTokenPresent {
		scopes, err := validateUserTokenScopes(*twitchAccessToken)
		if err != nil {
			logger.Warn("failed to validate user token scopes", "err", err)
		} else {
			validatedScopes = scopes
		}
	}
	collector.SetKnownOAuthScopes(collector.KnownUserScopes, validatedScopes)
	collector.SetCapabilities(appTokenPresent, userTokenPresent, validatedScopes)

	if err := collector.SetRewardGrouping(*rewardGroupDefault, *rewardGroupUnknown, *rewardGroupMax, map[string]string(*rewardGroupByID), map[string]string(*rewardGroupByTitle)); err != nil {
		logger.Error("invalid reward group configuration", "err", err)
		os.Exit(1)
	}

	if *twitchClientID != "" && *twitchClientSecret != "" {
		logger.Info("client type determined", "clientType", clientType)

		switch clientType {
		case "app":
			client, err = newClientWithSecret(logger, newInstrumentedHTTPClient("helix"))
			if err != nil {
				logger.Error("Error creating the client", "err", err)
				collector.DisableDefaultCollectors()
				configured.Set(0)
				client = nil
			}
		case "user":
			client, err = newClientWithUserAccessToken(logger, newInstrumentedHTTPClient("helix"))
			if err != nil {
				logger.Error("Error creating the client", "err", err)
				collector.DisableDefaultCollectors()
				configured.Set(0)
				client = nil
			}
		}
	}

	var eventsubClient *eventsub.Client

	if *eventSubEnabled {
		if client == nil {
			logger.Error("eventsub enabled but Twitch client is not configured; disabling eventsub")
		} else {
			logger.Info("eventsub endpoint enabled", "endpoint", "/eventsub")

			var appClient *helix.Client

			// eventsub requires an app client to create webhooks, but we may have created a user client
			// beforehand for subscription metrics, so just check and create the app client if needed
			if clientType == "user" {
				appClient, err = newClientWithSecret(logger, newInstrumentedHTTPClient("eventsub"))
				if err != nil {
					logger.Error("Error creating the client", "err", err)
					os.Exit(1)
				}
			} else {
				// Create a dedicated client so EventSub subscription management is tracked separately.
				appClient, err = newClientWithSecret(logger, newInstrumentedHTTPClient("eventsub"))
				if err != nil {
					logger.Error("Error creating eventsub client", "err", err)
					appClient = nil
				}
			}

			if *eventSubWebhookURL == "" || *eventSubWebhookSecret == "" {
				logger.Error("eventsub enabled but webhook URL/secret are missing; disabling eventsub")
				appClient = nil
			}
			if appClient != nil {

				var userClient *helix.Client
				if clientType == "user" {
					userClient = client
				}

				eventsubClient, err = eventsub.New(
					*twitchClientID,
					*twitchClientSecret,
					*eventSubWebhookURL,
					*eventSubWebhookSecret,
					logger,
					appClient,
					userClient,
				)

				if err != nil {
					logger.Error("Error creating the eventsub client", "err", err)
					eventsubClient = nil
				} else {
					eventsubClient.SetSignatureFailureHook(func(reason string) {
						collector.IncEventSubSignatureFail(reason)
					})
					// expose the eventsub endpoint
					http.HandleFunc("/eventsub", eventsubClient.Handler())
				}
			}
		}
	}

	selfLogin := strings.TrimSpace(*twitchSelfChannel)
	legacyLogins := (*twitchChannel)
	if selfLogin == "" && len(legacyLogins) > 0 {
		// Backwards-compatible default: treat first legacy channel as self.
		selfLogin = legacyLogins[0]
		if len(legacyLogins) > 1 {
			logger.Warn("multiple --twitch.channel values detected; treating first as self and the rest as watch; prefer --twitch.self-channel/--twitch.watch-channel")
		}
	}

	watchLogins := make([]string, 0, len(*twitchWatchChannels)+max(0, len(legacyLogins)-1))
	watchLogins = append(watchLogins, (*twitchWatchChannels)...)
	if len(legacyLogins) > 1 {
		watchLogins = append(watchLogins, legacyLogins[1:]...)
	}

	watchlist, wlErr := collector.NewChannelWatchlist(selfLogin, watchLogins)
	if wlErr != nil {
		logger.Error("invalid watchlist configuration", "err", wlErr)
		os.Exit(1)
	}

	exporter, err := collector.NewExporter(logger, client, eventsubClient, watchlist)
	if err != nil {
		logger.Error("Error creating the exporter", "err", err)
		collector.DisableDefaultCollectors()
		exporter, err = collector.NewExporter(logger, nil, nil, watchlist)
		if err != nil {
			logger.Error("Error creating fallback exporter", "err", err)
			os.Exit(1)
		}
	}

	r := prometheus.NewRegistry()
	r.MustRegister(configured)
	r.MustRegister(exporter)

	http.Handle(*metricsPath, promhttp.HandlerFor(r, promhttp.HandlerOpts{
		ErrorLog:      promHTTPLogger{logger: logger},
		ErrorHandling: promhttp.ContinueOnError,
	}))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`<html>
             <head><title>Twitch Exporter</title></head>
             <body>
             <h1>Twitch Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             <h2>Build</h2>
             <pre>` + version.Info() + ` ` + version.BuildContext() + `</pre>
             </body>
             </html>`))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	srv := &http.Server{}
	if err := web.ListenAndServe(srv, webConfig, logger); err != nil {
		logger.Error("Error starting HTTP server", "err", err)
		os.Exit(1)
	}
}

type validateTokenResponse struct {
	ClientID  string   `json:"client_id"`
	Login     string   `json:"login"`
	UserID    string   `json:"user_id"`
	Scopes    []string `json:"scopes"`
	ExpiresIn int      `json:"expires_in"`
}

func validateUserTokenScopes(accessToken string) ([]string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://id.twitch.tv/oauth2/validate", nil)
	if err != nil {
		return nil, err
	}
	// Twitch validate endpoint expects: Authorization: OAuth <token>
	req.Header.Set("Authorization", "OAuth "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("validate token returned status %d", resp.StatusCode)
	}

	var v validateTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, err
	}

	return v.Scopes, nil
}

func refreshAppAccessToken(logger *slog.Logger, client *helix.Client) {
	logger.Info("Refreshing app access token")
	appAccessToken, err := client.RequestAppAccessToken([]string{})
	if err != nil {
		logger.Error("Error getting app access token", "err", err)
		return
	}

	if appAccessToken.ErrorStatus != 0 {
		logger.Error("Error getting app access token", "err", appAccessToken.ErrorMessage)
		return
	}

	client.SetAppAccessToken(appAccessToken.Data.AccessToken)
}

func refreshUserAccessToken(logger *slog.Logger, client *helix.Client) {
	logger.Info("Refreshing user access token")
	userAccessToken, err := client.RefreshUserAccessToken(client.GetRefreshToken())
	if err != nil {
		logger.Error("Error getting user access token", "err", err)
		return
	}

	if userAccessToken.ErrorStatus != 0 {
		logger.Error("Error getting user access token", "err", userAccessToken.ErrorMessage)
		return
	}

	client.SetUserAccessToken(userAccessToken.Data.AccessToken)
}

// newClientWithSecret creates a new Twitch client with the use of an app access
// token.
func newClientWithSecret(logger *slog.Logger, httpClient helix.HTTPClient) (*helix.Client, error) {
	client, err := helix.NewClient(&helix.Options{
		ClientID:     *twitchClientID,
		ClientSecret: *twitchClientSecret,
		HTTPClient:   httpClient,
	})

	if err != nil {
		logger.Error("could not initialise twitch client", "err", err)
		return nil, err
	}

	refreshAppAccessToken(logger, client)

	refreshTicker := time.NewTicker(24 * time.Hour)
	go func(logger *slog.Logger, refreshTicker *time.Ticker, client *helix.Client) {
		for range refreshTicker.C {
			refreshAppAccessToken(logger, client)
		}
	}(logger, refreshTicker, client)

	return client, nil
}

// newClientWithUserAccessToken creates a new Twitch client with a user access token.
// this is required for private data, such as subscriber counts.
func newClientWithUserAccessToken(logger *slog.Logger, httpClient helix.HTTPClient) (*helix.Client, error) {
	// providing a refresh token allows the helix client to refresh the access
	// token when it expires. this is done automatically when using the helix
	// client.
	client, err := helix.NewClient(&helix.Options{
		ClientID:        *twitchClientID,
		ClientSecret:    *twitchClientSecret,
		UserAccessToken: *twitchAccessToken,
		RefreshToken:    *twitchRefreshToken,
		HTTPClient:      httpClient,
	})

	if err != nil {
		logger.Error("Error creating the client", "err", err)
		return nil, err
	}

	// it may be redundant to refresh the access token here, but it's done
	// anyway to ensure the access token is always valid, in case the parameters
	// are outdated
	refreshUserAccessToken(logger, client)

	refreshTicker := time.NewTicker(24 * time.Hour)
	go func(logger *slog.Logger, refreshTicker *time.Ticker, client *helix.Client) {
		for range refreshTicker.C {
			refreshUserAccessToken(logger, client)
		}
	}(logger, refreshTicker, client)

	return client, nil
}

type instrumentedHTTPClient struct {
	api   string
	inner *http.Client
}

func newInstrumentedHTTPClient(api string) helix.HTTPClient {
	return &instrumentedHTTPClient{
		api: api,
		inner: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *instrumentedHTTPClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.inner.Do(req)
	if err != nil {
		return nil, err
	}
	collector.ObserveAPIResponse(c.api, endpointLabel(req.URL.Path), resp.StatusCode, resp.Header)
	return resp, nil
}

func endpointLabel(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return "/"
	}
	// Keep it bounded: paths are stable, IDs are in query params for Helix.
	return path
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

package eventsub

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/LinneB/twitchwh"
	"github.com/nicklaw5/helix/v2"
)

type ChannelChatMessageEvent struct {
	BroadcasterUserID       string `json:"broadcaster_user_id"`
	BroadcasterUserLogin    string `json:"broadcaster_user_login"`
	BroadcasterUserName     string `json:"broadcaster_user_name"`
	SourceBroadcasterUserID string `json:"source_broadcaster_user_id"`
	SourceBroadcasterLogin  string `json:"source_broadcaster_user_login"`
	SourceBroadcasterName   string `json:"source_broadcaster_user_name"`
	ChatterUserID           string `json:"chatter_user_id"`
	ChatterUserLogin        string `json:"chatter_user_login"`
	ChatterUserName         string `json:"chatter_user_name"`
	MessageID               string `json:"message_id"`
	SourceMessageID         string `json:"source_message_id"`
	IsSourceOnly            bool   `json:"is_source_only"`
	Message                 struct {
		Text      string `json:"text"`
		Fragments []struct {
			Type      string      `json:"type"`
			Text      string      `json:"text"`
			Cheermote interface{} `json:"cheermote"`
			Emote     interface{} `json:"emote"`
			Mention   interface{} `json:"mention"`
		} `json:"fragments"`
	} `json:"message"`
	Color                       string  `json:"color"`
	Badges                      []Badge `json:"badges"`
	SourceBadges                []Badge `json:"source_badges"`
	MessageType                 string  `json:"message_type"`
	Cheer                       string  `json:"cheer"`
	Reply                       string  `json:"reply"`
	ChannelPointsCustomRewardID string  `json:"channel_points_custom_reward_id"`
	ChannelPointsAnimationID    string  `json:"channel_points_animation_id"`
}

type Badge struct {
	SetID string `json:"set_id"`
	ID    string `json:"id"`
	Info  string `json:"info"`
}

var ErrEventsubClientNotSet = errors.New("eventsub client not set")

type Client struct {
	webhookURL    string
	webhookSecret string

	appClient  *helix.Client
	userClient *helix.Client
	logger     *slog.Logger
	cl         *twitchwh.Client

	onSignatureFailure func(reason string)
}

func New(
	clientID, clientSecret, webhookURL, webhookSecret string,
	logger *slog.Logger,
	appClient *helix.Client,
	userClient *helix.Client,
) (*Client, error) {
	eventsubCl := &Client{
		appClient:     appClient,
		userClient:    userClient,
		logger:        logger,
		webhookURL:    webhookURL,
		webhookSecret: webhookSecret,
	}

	cl, err := twitchwh.New(twitchwh.ClientConfig{
		ClientID:      clientID,
		ClientSecret:  clientSecret,
		WebhookURL:    webhookURL,
		WebhookSecret: webhookSecret,
		Debug:         true,
	})

	if err != nil {
		return nil, err
	}

	eventsubCl.cl = cl

	return eventsubCl, nil
}

func (c *Client) SetSignatureFailureHook(hook func(reason string)) {
	c.onSignatureFailure = hook
}

func (c *Client) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Verify Twitch signature before delegating to twitchwh.
		body, err := io.ReadAll(r.Body)
		if err != nil {
			c.logger.Warn("failed to read eventsub request body", "err", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = r.Body.Close()

		if !c.verifySignature(r.Header, body) {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		// Recreate request body for downstream handler.
		r.Body = io.NopCloser(bytes.NewReader(body))
		c.cl.Handler(w, r)
	}
}

func (c *Client) verifySignature(headers http.Header, body []byte) bool {
	msgID := headers.Get("Twitch-Eventsub-Message-Id")
	timestamp := headers.Get("Twitch-Eventsub-Message-Timestamp")
	sig := headers.Get("Twitch-Eventsub-Message-Signature")
	if msgID == "" || timestamp == "" || sig == "" {
		if c.onSignatureFailure != nil {
			c.onSignatureFailure("missing_headers")
		}
		return false
	}

	// Signature is sha256=hex
	const prefix = "sha256="
	if len(sig) <= len(prefix) || sig[:len(prefix)] != prefix {
		if c.onSignatureFailure != nil {
			c.onSignatureFailure("bad_signature")
		}
		return false
	}

	provided, err := hex.DecodeString(sig[len(prefix):])
	if err != nil {
		if c.onSignatureFailure != nil {
			c.onSignatureFailure("bad_signature")
		}
		return false
	}

	mac := hmac.New(sha256.New, []byte(c.webhookSecret))
	mac.Write([]byte(msgID))
	mac.Write([]byte(timestamp))
	mac.Write(body)
	expected := mac.Sum(nil)

	if !hmac.Equal(provided, expected) {
		if c.onSignatureFailure != nil {
			c.onSignatureFailure("bad_signature")
		}
		return false
	}

	return true
}

func (c *Client) On(event string, callback func(eventRaw json.RawMessage)) error {
	// juuust in case
	if c.cl == nil {
		c.logger.Warn("eventsub client not set")
		return ErrEventsubClientNotSet
	}

	c.cl.On(event, callback)
	return nil
}

func (c *Client) SubscribeApp(eventType string, version string, condition helix.EventSubCondition) error {
	return c.subscribeWithClient(c.appClient, eventType, version, condition)
}

func (c *Client) SubscribeUser(eventType string, version string, condition helix.EventSubCondition) error {
	if c.userClient == nil {
		return errors.New("user client not configured")
	}
	return c.subscribeWithClient(c.userClient, eventType, version, condition)
}

func (c *Client) subscribeWithClient(client *helix.Client, eventType string, version string, condition helix.EventSubCondition) error {
	if c.cl == nil {
		c.logger.Warn("eventsub client not set")
		return ErrEventsubClientNotSet
	}
	if client == nil {
		return errors.New("helix client not configured")
	}

	c.logger.Info("subscribing to event", "event", eventType)

	// cannot filter by both the user id and the event type, so the better option is to get all the user
	// subscriptions and see if the event type is found already
	filterUserID := condition.BroadcasterUserID
	if filterUserID == "" {
		filterUserID = condition.UserID
	}
	subscriptions, err := client.GetEventSubSubscriptions(&helix.EventSubSubscriptionsParams{
		Type:   eventType,
		UserID: filterUserID,
	})

	if err != nil {
		return err
	}

	for _, v := range subscriptions.Data.EventSubSubscriptions {
		if v.Type != eventType {
			continue
		}
		if v.Transport.Callback != c.webhookURL {
			continue
		}
		if v.Condition != condition {
			continue
		}
		if v.Status == "enabled" || v.Status == "webhook_callback_verification_pending" {
			c.logger.Info("subscription already exists", "event", eventType, "status", v.Status)
			return nil
		}
	}

	if version == "" {
		version = "1"
	}
	res, err := client.CreateEventSubSubscription(&helix.EventSubSubscription{
		Type:      eventType,
		Version:   version,
		Condition: condition,
		Transport: helix.EventSubTransport{
			Method:   "webhook",
			Callback: c.webhookURL,
			Secret:   c.webhookSecret,
		},
	})

	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusAccepted {
		c.logger.Info("failed to create subscription", "res", res)
		return errors.Join(errors.New("failed to create subscription"), errors.New(res.ErrorMessage))
	}

	c.logger.Info("subscription created", "error", res.Error, "status_code", res.StatusCode, "data", res.Data)

	// c.logger.Info("subscription created", "event", eventType, "broadcaster_id", broadcasterID, "subscription_id", res.Data.EventSubSubscriptions[0].ID)

	return nil
}

func (c *Client) ListSubscriptions() ([]helix.EventSubSubscription, error) {
	if c.appClient == nil {
		return nil, errors.New("app client not configured")
	}
	res, err := c.appClient.GetEventSubSubscriptions(&helix.EventSubSubscriptionsParams{})
	if err != nil {
		return nil, err
	}
	return res.Data.EventSubSubscriptions, nil
}

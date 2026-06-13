package provider

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/golang-jwt/jwt/v5"
	"github.com/onix-fun/push-service/internal/config"
	"github.com/onix-fun/push-service/internal/model"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var ErrPermanent = errors.New("permanent provider error")

type Gateway struct {
	cfg       config.ProviderConfig
	http      *http.Client
	fcmTokens oauth2.TokenSource
	apnsKey   *ecdsa.PrivateKey
}

func New(ctx context.Context, cfg config.ProviderConfig) (*Gateway, error) {
	gateway := &Gateway{cfg: cfg, http: &http.Client{Timeout: 10 * time.Second}}
	if cfg.FCM.Enabled {
		credentials, err := google.CredentialsFromJSON(ctx, []byte(cfg.FCM.Credentials), "https://www.googleapis.com/auth/firebase.messaging")
		if err != nil {
			return nil, fmt.Errorf("parse FCM credentials: %w", err)
		}
		gateway.fcmTokens = credentials.TokenSource
	}
	if cfg.APNS.Enabled {
		block, _ := pem.Decode([]byte(cfg.APNS.PrivateKey))
		if block == nil {
			return nil, errors.New("decode APNs private key: no PEM block")
		}
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse APNs private key: %w", err)
		}
		var ok bool
		gateway.apnsKey, ok = key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, errors.New("parse APNs private key: expected ECDSA key")
		}
	}
	return gateway, nil
}

func (g *Gateway) Send(ctx context.Context, device model.Device, command model.Command) error {
	switch device.Provider {
	case "web_push":
		return g.sendWebPush(ctx, device, command)
	case "fcm":
		return g.sendFCM(ctx, device, command)
	case "apns":
		return g.sendAPNS(ctx, device, command)
	default:
		return fmt.Errorf("%w: unknown provider", ErrPermanent)
	}
}

func (g *Gateway) sendWebPush(ctx context.Context, device model.Device, command model.Command) error {
	if !g.cfg.WebPush.Enabled {
		return fmt.Errorf("%w: provider disabled", ErrPermanent)
	}
	var subscription webpush.Subscription
	if err := json.Unmarshal([]byte(device.Token), &subscription); err != nil {
		return fmt.Errorf("%w: invalid subscription", ErrPermanent)
	}
	payload, _ := json.Marshal(map[string]any{"title": command.Title, "body": command.Body, "data": command.Data})
	response, err := webpush.SendNotificationWithContext(ctx, payload, &subscription, &webpush.Options{
		Subscriber:      g.cfg.WebPush.Subject,
		VAPIDPublicKey:  g.cfg.WebPush.PublicKey,
		VAPIDPrivateKey: g.cfg.WebPush.PrivateKey,
		TTL:             command.TTL,
	})
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return classifyResponse("web push", response)
}

func (g *Gateway) sendFCM(ctx context.Context, device model.Device, command model.Command) error {
	if !g.cfg.FCM.Enabled {
		return fmt.Errorf("%w: provider disabled", ErrPermanent)
	}
	token, err := g.fcmTokens.Token()
	if err != nil {
		return fmt.Errorf("get FCM access token: %w", err)
	}
	data := make(map[string]string, len(command.Data))
	for key, value := range command.Data {
		switch typed := value.(type) {
		case string:
			data[key] = typed
		default:
			encoded, _ := json.Marshal(typed)
			data[key] = string(encoded)
		}
	}
	message := map[string]any{
		"token":        device.Token,
		"notification": map[string]string{"title": command.Title, "body": command.Body},
		"data":         data,
	}
	if command.TTL > 0 || command.CollapseKey != "" {
		message["android"] = map[string]any{"ttl": fmt.Sprintf("%ds", command.TTL), "collapse_key": command.CollapseKey}
	}
	endpoint := g.cfg.FCM.Endpoint
	if endpoint == "" {
		endpoint = "https://fcm.googleapis.com/v1/projects/" + url.PathEscape(g.cfg.FCM.ProjectID) + "/messages:send"
	}
	response, err := g.post(ctx, endpoint, map[string]any{"message": message}, map[string]string{"Authorization": "Bearer " + token.AccessToken})
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return classifyResponse("FCM", response)
}

func (g *Gateway) sendAPNS(ctx context.Context, device model.Device, command model.Command) error {
	if !g.cfg.APNS.Enabled {
		return fmt.Errorf("%w: provider disabled", ErrPermanent)
	}
	now := time.Now()
	auth := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.RegisteredClaims{
		Issuer:   g.cfg.APNS.TeamID,
		IssuedAt: jwt.NewNumericDate(now),
	})
	auth.Header["kid"] = g.cfg.APNS.KeyID
	signed, err := auth.SignedString(g.apnsKey)
	if err != nil {
		return fmt.Errorf("sign APNs token: %w", err)
	}
	endpoint := strings.TrimSuffix(g.cfg.APNS.Endpoint, "/")
	if endpoint == "" {
		endpoint = "https://api.push.apple.com"
	}
	headers := map[string]string{
		"Authorization": "bearer " + signed,
		"apns-topic":    g.cfg.APNS.BundleID,
	}
	if command.CollapseKey != "" {
		headers["apns-collapse-id"] = command.CollapseKey
	}
	if command.TTL > 0 {
		headers["apns-expiration"] = fmt.Sprintf("%d", now.Add(time.Duration(command.TTL)*time.Second).Unix())
	}
	payload := map[string]any{"aps": map[string]any{"alert": map[string]string{"title": command.Title, "body": command.Body}}, "data": command.Data}
	response, err := g.post(ctx, endpoint+"/3/device/"+url.PathEscape(device.Token), payload, headers)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return classifyResponse("APNs", response)
}

func (g *Gateway) post(ctx context.Context, endpoint string, payload any, headers map[string]string) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	return g.http.Do(request)
}

func classifyResponse(name string, response *http.Response) error {
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	message := fmt.Sprintf("%s HTTP %d: %s", name, response.StatusCode, body)
	permanentBody := strings.Contains(string(body), "UNREGISTERED") ||
		strings.Contains(string(body), "BadDeviceToken") ||
		strings.Contains(string(body), "DeviceTokenNotForTopic")
	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusGone || permanentBody {
		return fmt.Errorf("%w: %s", ErrPermanent, message)
	}
	return errors.New(message)
}

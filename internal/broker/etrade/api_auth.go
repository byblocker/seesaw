package etrade

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"seesaw/internal/broker"
	"seesaw/internal/keyring"
	"time"

	"github.com/dghubble/oauth1"
	"github.com/go-resty/resty/v2"
)

const (
	etradeRequestTokenURL = "https://api.etrade.com/oauth/request_token"
	etradeAuthorizeURL    = "https://us.etrade.com/e/t/etws/authorize"
	etradeAccessTokenURL  = "https://api.etrade.com/oauth/access_token"
	etradeRenewAccessURL  = "https://api.etrade.com/oauth/renew_access_token"

	accessTokenKey = "etrade_oauth_token"

	// ETrade expires all access tokens at midnight EST.
	timezone = "America/New_York"

	// ETrade deactivates access tokens after two hours of inactivity.
	etradeMaxAccessTokenAge = 2 * time.Hour
)

type oauthSession struct {
	ConfigurationHash [32]byte      `json:"configuration_hash"`
	Token             *oauth1.Token `json:"token"`
	LastRefreshed     time.Time     `json:"last_used"`
	ExpiresAt         time.Time     `json:"expires_at"`
}

func authenticate(ctx context.Context, key, secret, baseURL string) (*resty.Client, error) {
	oauth := &oauth1.Config{
		ConsumerKey:    key,
		ConsumerSecret: secret,
		CallbackURL:    "oob",
		HTTPClient:     &http.Client{Timeout: broker.Timeout},
		Endpoint: oauth1.Endpoint{
			RequestTokenURL: etradeRequestTokenURL,
			AuthorizeURL:    etradeAuthorizeURL,
			AccessTokenURL:  etradeAccessTokenURL,
		},
	}

	// Bind tokens to the credentials and endpoint.
	configuration, err := json.Marshal([3]string{key, secret, baseURL})
	if err != nil {
		return nil, fmt.Errorf("could not identify ETrade configuration: %w", err)
	}
	configurationHash := sha256.Sum256(configuration)
	clear(configuration)
	stop := broker.StartTimer("Connecting to %s", Name)
	client, err := refreshETradeClient(ctx, oauth, configurationHash)
	stop(err)
	if err != nil {
		log.Printf("CANNOT renew %s client: %v", Name, err)

		client, err = authNewETradeClient(ctx, oauth, configurationHash)
	}
	if err != nil {
		return nil, err
	}

	return client, nil
}

func saveOAuthToken(token *oauth1.Token, refreshedAt time.Time, configurationHash [32]byte) error {
	expiresAt, err := endOfDay(refreshedAt)
	if err != nil {
		return err
	}

	return keyring.Set(accessTokenKey, oauthSession{
		ConfigurationHash: configurationHash,
		Token:             token,
		LastRefreshed:     refreshedAt,
		ExpiresAt:         expiresAt,
	})
}

func endOfDay(moment time.Time) (time.Time, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timezone %q: %w", timezone, err)
	}
	year, month, day := moment.In(location).Date()
	return time.Date(year, month, day, 23, 59, 59, 0, location), nil
}

func refreshETradeClient(ctx context.Context, oauth *oauth1.Config, configurationHash [32]byte) (*resty.Client, error) {
	session, err := keyring.Get[oauthSession](accessTokenKey)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, fmt.Errorf("no saved token")
	}
	if err != nil {
		return nil, err
	}

	if session.ConfigurationHash != configurationHash || session.Token == nil || session.Token.Token == "" || session.Token.TokenSecret == "" {
		return nil, fmt.Errorf("saved token does not match ETrade configuration or is incomplete")
	}

	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("saved token expired")
	}

	client := broker.NewHTTPClient(oauth.Client(ctx, session.Token))

	if time.Since(session.LastRefreshed) > etradeMaxAccessTokenAge {
		// No need to rotate the secret, ETrade just needs a keep-alive.
		_, err := client.R().SetContext(ctx).Get(etradeRenewAccessURL)
		if err != nil {
			return nil, fmt.Errorf("failed to renew token: %w", err)
		}

		if err := saveOAuthToken(session.Token, time.Now(), configurationHash); err != nil {
			return nil, err
		}
	}

	return client, nil
}

func authNewETradeClient(ctx context.Context, oauth *oauth1.Config, configurationHash [32]byte) (*resty.Client, error) {
	requestToken, requestSecret, err := oauth.RequestToken()
	if err != nil {
		return nil, fmt.Errorf("failed to obtain request token: %w", err)
	}

	authURL := fmt.Sprintf("%s?%s", oauth.Endpoint.AuthorizeURL, url.Values{
		"key":   {oauth.ConsumerKey},
		"token": {requestToken},
	}.Encode())

	if _, err := broker.AskForInput(fmt.Sprintf("Sign in to %s. Press ENTER to open the sign-in page in your browser. ", Name)); err != nil {
		return nil, fmt.Errorf("failed to wait for ENTER: %w", err)
	}
	if err := broker.OpenURL(authURL); err != nil {
		fmt.Fprintf(os.Stderr, "%v; open this link instead:\n%s\n", err, authURL)
	}

	verificationCode, err := broker.AskForInput("Enter verification code: ")
	if err != nil {
		return nil, fmt.Errorf("failed to read verification code: %w", err)
	}

	stop := broker.StartTimer("Creating %s access token", Name)
	accessToken, accessSecret, err := oauth.AccessToken(requestToken, requestSecret, verificationCode)
	stop(err)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain access token: %w", err)
	}

	token := oauth1.NewToken(accessToken, accessSecret)
	if err := saveOAuthToken(token, time.Now(), configurationHash); err != nil {
		return nil, fmt.Errorf("failed to save new access token: %w", err)
	}

	return broker.NewHTTPClient(oauth.Client(ctx, token)), nil
}

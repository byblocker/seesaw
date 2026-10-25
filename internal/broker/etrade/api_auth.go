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
	"seesaw/internal/calendar"
	"seesaw/internal/keyring"
	"seesaw/internal/utils"
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

type oauthCache struct {
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
		HTTPClient:     &http.Client{Timeout: utils.Timeout},
		Endpoint: oauth1.Endpoint{
			RequestTokenURL: etradeRequestTokenURL,
			AuthorizeURL:    etradeAuthorizeURL,
			AccessTokenURL:  etradeAccessTokenURL,
		},
	}

	// Bind tokens to the credentials and endpoint, including for explicit-config callers.
	configuration, err := json.Marshal([3]string{key, secret, baseURL})
	if err != nil {
		return nil, fmt.Errorf("could not identify ETrade configuration: %w", err)
	}
	configurationHash := sha256.Sum256(configuration)
	clear(configuration)
	client, err := refreshETradeClient(ctx, oauth, configurationHash)
	if err != nil {
		log.Printf("CANNOT renew %s client: %v", Name, err)

		client, err = authNewETradeClient(ctx, oauth, configurationHash)
	}
	if err != nil {
		return nil, err
	}

	return client, nil
}

func cacheOAuthToken(token *oauth1.Token, refreshedAt time.Time, configurationHash [32]byte) error {
	expiresAt, err := calendar.EndOfDay(refreshedAt, timezone)
	if err != nil {
		return err
	}

	return keyring.Set(accessTokenKey, oauthCache{
		ConfigurationHash: configurationHash,
		Token:             token,
		LastRefreshed:     refreshedAt,
		ExpiresAt:         expiresAt,
	})
}

func refreshETradeClient(ctx context.Context, oauth *oauth1.Config, configurationHash [32]byte) (*resty.Client, error) {
	access, err := keyring.Get[oauthCache](accessTokenKey)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, fmt.Errorf("no cached token")
	}
	if err != nil {
		return nil, err
	}

	if access.ConfigurationHash != configurationHash || access.Token == nil || access.Token.Token == "" || access.Token.TokenSecret == "" {
		return nil, fmt.Errorf("cached token does not match ETrade configuration or is incomplete")
	}

	if time.Now().After(access.ExpiresAt) {
		return nil, fmt.Errorf("cached token expired")
	}

	client := utils.NewHTTPClient(oauth.Client(ctx, access.Token))

	if time.Since(access.LastRefreshed) > etradeMaxAccessTokenAge {
		// No need to rotate the secret, ETrade just needs a keep-alive.
		stop := utils.StartTimer("Renewing %s access token", Name)
		_, err := client.R().SetContext(ctx).Get(etradeRenewAccessURL)
		stop(err)
		if err != nil {
			return nil, fmt.Errorf("failed to renew token: %w", err)
		}

		if err := cacheOAuthToken(access.Token, time.Now(), configurationHash); err != nil {
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

	if _, err := utils.AskForInput(fmt.Sprintf("Sign in to %s. Press ENTER to open the sign-in page in your browser. ", Name)); err != nil {
		return nil, fmt.Errorf("failed to wait for ENTER: %w", err)
	}
	if err := utils.OpenUrl(authURL); err != nil {
		fmt.Fprintf(os.Stderr, "%v; open this link instead:\n%s\n", err, authURL)
	}

	verificationCode, err := utils.AskForInput("Enter verification code: ")
	if err != nil {
		return nil, fmt.Errorf("failed to read verification code: %w", err)
	}

	stop := utils.StartTimer("Creating %s access token", Name)
	accessToken, accessSecret, err := oauth.AccessToken(requestToken, requestSecret, verificationCode)
	stop(err)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain access token: %w", err)
	}

	token := oauth1.NewToken(accessToken, accessSecret)
	if err := cacheOAuthToken(token, time.Now(), configurationHash); err != nil {
		return nil, fmt.Errorf("failed to cache new access token: %w", err)
	}

	return utils.NewHTTPClient(oauth.Client(ctx, token)), nil
}

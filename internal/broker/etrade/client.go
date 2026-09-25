package etrade

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"seesaw/internal/broker"
	"seesaw/internal/httpclient"
	"seesaw/internal/keyring"
	"seesaw/internal/utils"

	"time"

	"github.com/dghubble/oauth1"
)

const (
	etradeRequestTokenURL = "https://api.etrade.com/oauth/request_token"
	etradeAuthorizeURL    = "https://us.etrade.com/e/t/etws/authorize"
	oauthCacheTokenURL    = "https://api.etrade.com/oauth/access_token"
	etradeRenewAccessURL  = "https://api.etrade.com/oauth/renew_access_token"

	keyringService = "seesaw"
	accessTokenKey = "etrade_oauth_token"

	// ETrade expires all access tokens at midnight EST.
	timezone = "America/New_York"

	// ETrade deactivates access tokens after two hours of inactivity.
	etradeMaxAccessTokenAge = 2 * time.Hour
)

var _ broker.Broker = (*Client)(nil)

func (client *Client) Settings() broker.Settings {
	return broker.Settings{
		AllowFractionalShares: false,
	}
}

type Client struct {
	httpClient *http.Client `yaml:"-"`
	apiBaseURL string       `yaml:"api_base_url"`
}

type oauthCache struct {
	Token     *oauth1.Token `json:"token"`
	LastUsed  time.Time     `json:"last_used"`
	ExpiresAt time.Time     `json:"expires_at"`
}

func NewClient(ctx context.Context, key, secret, baseURL string) (*Client, error) {
	oauth := &oauth1.Config{
		ConsumerKey:    key,
		ConsumerSecret: secret,
		CallbackURL:    "oob",
		Endpoint: oauth1.Endpoint{
			RequestTokenURL: etradeRequestTokenURL,
			AuthorizeURL:    etradeAuthorizeURL,
			AccessTokenURL:  oauthCacheTokenURL,
		},
	}

	client, err := refreshETradeClient(ctx, oauth)
	if err != nil {
		log.Printf("Could not renew ETrade client: %v", err)

		client, err = authNewETradeClient(ctx, oauth)
		if err != nil {
			return nil, err
		}
	}

	return &Client{httpClient: client, apiBaseURL: baseURL}, nil
}

func cacheOAuthToken(token *oauth1.Token, lastUsed time.Time) error {
	expiresAt, err := utils.EndOfDay(lastUsed, timezone)
	if err != nil {
		return err
	}

	return keyring.Set(keyringService, accessTokenKey, oauthCache{
		Token:     token,
		LastUsed:  lastUsed,
		ExpiresAt: expiresAt,
	})
}

func refreshETradeClient(ctx context.Context, oauth *oauth1.Config) (*http.Client, error) {
	log.Print("Attempting to renew ETrade credentials...")

	access, err := keyring.Get[oauthCache](keyringService, accessTokenKey)
	if err != nil {
		return nil, err
	}

	if time.Now().After(access.ExpiresAt) {
		return nil, fmt.Errorf("token expired yesterday")
	}

	client := oauth.Client(ctx, access.Token)

	if time.Since(access.LastUsed) > etradeMaxAccessTokenAge {
		// No need to rotate the secret, ETrade just needs a keep-alive.
		if _, err := httpclient.SendHTTPRequest(ctx, client, http.MethodGet, etradeRenewAccessURL, nil); err != nil {
			return nil, fmt.Errorf("failed to renew token: %v", err)
		}

		if err := cacheOAuthToken(access.Token, time.Now()); err != nil {
			return nil, err
		}
	}

	log.Print("Successfully renewed ETrade client!")
	return client, nil
}

func authNewETradeClient(ctx context.Context, oauth *oauth1.Config) (*http.Client, error) {
	log.Print("Reauthorizing with ETrade...")

	requestToken, requestSecret, err := oauth.RequestToken()
	if err != nil {
		return nil, fmt.Errorf("failed to obtain request token: %v", err)
	}

	authURL := fmt.Sprintf("%s?%s", oauth.Endpoint.AuthorizeURL, url.Values{
		"key":   {oauth.ConsumerKey},
		"token": {requestToken},
	}.Encode())

	log.Print("Redirecting to authenticate with ETrade...")
	if err = utils.OpenUrl(authURL); err != nil {
		return nil, err
	}

	verificationCode, err := utils.AskForInput("Enter verification code: ")
	if err != nil {
		return nil, fmt.Errorf("failed to read verification code: %v", err)
	}

	log.Print("Received verification code. Creating new access token...")
	accessToken, accessSecret, err := oauth.AccessToken(requestToken, requestSecret, verificationCode)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain access token: %v", err)
	}

	token := oauth1.NewToken(accessToken, accessSecret)
	if err := cacheOAuthToken(token, time.Now()); err != nil {
		return nil, fmt.Errorf("failed to cache new access token: %v", err)
	}

	return oauth.Client(ctx, token), nil
}

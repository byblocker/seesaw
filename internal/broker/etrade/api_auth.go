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
	"seesaw/pkg/secrets"
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
	CredentialsHash [32]byte      `json:"configuration_hash"`
	Token           *oauth1.Token `json:"token"`
	LastRefreshed   time.Time     `json:"last_used"`
	ExpiresAt       time.Time     `json:"expires_at"`
}

type signIn struct {
	store           secrets.Store
	oauth           *oauth1.Config
	credentialsHash [32]byte
}

func authenticate(ctx context.Context, store secrets.Store, credentials Credentials) (*resty.Client, error) {
	oauth := &oauth1.Config{
		ConsumerKey:    credentials.APIKey,
		ConsumerSecret: credentials.APISecret,
		CallbackURL:    "oob",
		HTTPClient:     &http.Client{Timeout: broker.Timeout},
		Endpoint: oauth1.Endpoint{
			RequestTokenURL: etradeRequestTokenURL,
			AuthorizeURL:    etradeAuthorizeURL,
			AccessTokenURL:  etradeAccessTokenURL,
		},
	}

	// Bind tokens to the credentials and endpoint.
	encoded, err := json.Marshal([3]string{credentials.APIKey, credentials.APISecret, credentials.APIBaseURL})
	if err != nil {
		return nil, fmt.Errorf("could not identify ETrade configuration: %w", err)
	}
	auth := signIn{store: store, oauth: oauth, credentialsHash: sha256.Sum256(encoded)}
	clear(encoded)

	stop := broker.StartTimer("Connecting to %s", Name)
	client, err := auth.refresh(ctx)
	stop(err)
	if err != nil {
		log.Printf("CANNOT renew %s client: %v", Name, err)

		client, err = auth.authorize(ctx)
	}
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (auth signIn) saveToken(ctx context.Context, token *oauth1.Token, refreshedAt time.Time) error {
	expiresAt, err := endOfDay(refreshedAt)
	if err != nil {
		return err
	}

	return saveSecret(ctx, auth.store, accessTokenKey, oauthSession{
		CredentialsHash: auth.credentialsHash,
		Token:           token,
		LastRefreshed:   refreshedAt,
		ExpiresAt:       expiresAt,
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

func (auth signIn) refresh(ctx context.Context) (*resty.Client, error) {
	saved, err := loadSecret[oauthSession](ctx, auth.store, accessTokenKey)
	if errors.Is(err, secrets.ErrNotFound) {
		return nil, fmt.Errorf("no saved token")
	}
	if err != nil {
		return nil, err
	}

	if saved.CredentialsHash != auth.credentialsHash || saved.Token == nil || saved.Token.Token == "" || saved.Token.TokenSecret == "" {
		return nil, fmt.Errorf("saved token does not match ETrade configuration or is incomplete")
	}

	if time.Now().After(saved.ExpiresAt) {
		return nil, fmt.Errorf("saved token expired")
	}

	client := broker.NewHTTPClient(auth.oauth.Client(ctx, saved.Token))

	if time.Since(saved.LastRefreshed) > etradeMaxAccessTokenAge {
		// No need to rotate the secret, ETrade just needs a keep-alive.
		_, err := client.R().SetContext(ctx).Get(etradeRenewAccessURL)
		if err != nil {
			return nil, fmt.Errorf("failed to renew token: %w", err)
		}

		if err := auth.saveToken(ctx, saved.Token, time.Now()); err != nil {
			return nil, err
		}
	}

	return client, nil
}

func (auth signIn) authorize(ctx context.Context) (*resty.Client, error) {
	oauth := auth.oauth
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
	if err := auth.saveToken(ctx, token, time.Now()); err != nil {
		return nil, fmt.Errorf("failed to save new access token: %w", err)
	}

	return broker.NewHTTPClient(oauth.Client(ctx, token)), nil
}

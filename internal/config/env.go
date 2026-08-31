package config

import (
	"fmt"
	"os"

	_ "github.com/joho/godotenv/autoload"
)

type ENV struct {
	ETradeAPIBaseURL string
	ETradeApiKey     string
	ETradeApiSecret  string
}

func LoadEnv() (*ENV, error) {
	env := &ENV{}
	fields := map[string]*string{
		"ETRADE_API_BASE_URL": &env.ETradeAPIBaseURL,
		"ETRADE_API_KEY":      &env.ETradeApiKey,
		"ETRADE_API_SECRET":   &env.ETradeApiSecret,
	}

	for key, field := range fields {
		if *field = os.Getenv(key); *field == "" {
			return nil, fmt.Errorf("%s cannot be empty or unset", key)
		}
	}

	return env, nil
}

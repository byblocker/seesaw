package services

import (
	"os"

	_ "github.com/joho/godotenv/autoload"
)

type ENV struct {
	ETradeAPIBaseURL string
	ETradeApiKey     string
	ETradeApiSecret  string
}

func LoadEnv() *ENV {
	return &ENV{
		ETradeAPIBaseURL: os.Getenv("ETRADE_API_BASE_URL"),
		ETradeApiKey:     os.Getenv("ETRADE_API_KEY"),
		ETradeApiSecret:  os.Getenv("ETRADE_API_SECRET"),
	}
}

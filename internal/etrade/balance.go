package etrade

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"seesaw/internal"

	"github.com/quagmt/udecimal"
)

type accountBalanceDTO struct {
	BalanceResponse struct {
		Id       string `json:"accountId"`
		Computed struct {
			BuyingPower float64 `json:"cashBuyingPower"`
		} `json:"Computed"`
	} `json:"BalanceResponse"`
}

func (client Client) GetAccountBalance(ctx context.Context, accountKey string) (udecimal.Decimal, error) {
	log.Print("Checking account's cash balance...")

	endpoint := fmt.Sprintf(
		"%s/accounts/%s/balance",
		client.APIBaseUrl,
		accountKey,
	)

	resp, err := internal.SendJSONRequest[accountBalanceDTO](
		ctx,
		client.httpClient,
		http.MethodGet,
		endpoint,
		nil,
		url.Values{
			"instType": {BrokerageAccountType},
			// Without this, ETrade returns a cached balance that can lag behind
			// actual settlemenet, causing the buying power to appear stale.
			"realTimeNAV": {"true"},
		},
	)
	if err != nil {
		return udecimal.Zero, fmt.Errorf("failed to load account balance: %w", err)
	}

	buyingPower := udecimal.MustFromFloat64(resp.BalanceResponse.Computed.BuyingPower)
	buyingPower = buyingPower.RoundBank(2)

	return buyingPower, nil
}

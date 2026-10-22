package etrade

import (
	"context"
	"fmt"
	"net/url"
	"seesaw/internal/utils"
	"seesaw/pkg/models"

	"github.com/shopspring/decimal"
)

type balanceResponse struct {
	BalanceResponse struct {
		Computed struct {
			NetCash     *decimal.Decimal `json:"netCash"`
			BuyingPower *decimal.Decimal `json:"cashBuyingPower"`
		} `json:"Computed"`
	} `json:"BalanceResponse"`
}

func (client *Client) Balances(ctx context.Context, account Account) (models.Balances, error) {
	if account.key == "" {
		return models.Balances{}, fmt.Errorf("expected an ETRADE account with a nonempty key")
	}

	endpoint := fmt.Sprintf("%s/accounts/%s/balance", client.apiBaseURL, url.PathEscape(account.key))
	query := url.Values{
		"instType":    {brokerageAccountType},
		"realTimeNAV": {"true"},
	}

	var timer utils.Timer
	timer.Start("Fetching account balances")
	response, err := utils.FetchJSON[balanceResponse](ctx, client.httpClient, endpoint+"?"+query.Encode())
	timer.Stop(err)
	if err != nil {
		return models.Balances{}, fmt.Errorf("fetch account balances: %w", err)
	}

	computed := response.BalanceResponse.Computed
	for _, field := range []struct {
		name  string
		value *decimal.Decimal
	}{
		{"netCash", computed.NetCash},
		{"cashBuyingPower", computed.BuyingPower},
	} {
		if field.value == nil {
			return models.Balances{}, fmt.Errorf("account balance response is missing %s", field.name)
		}
	}

	return models.Balances{
		CashBalance: *computed.NetCash,
		BuyingPower: *computed.BuyingPower,
	}, nil
}

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
			NetCash     *decimal.Decimal `json:"netCash" validate:"required"`
			BuyingPower *decimal.Decimal `json:"cashBuyingPower" validate:"required"`
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

	stop := utils.StartTimer("Fetching account balances")
	response, err := utils.FetchJSON[balanceResponse](ctx, client.httpClient, endpoint+"?"+query.Encode())
	stop(err)
	if err != nil {
		return models.Balances{}, fmt.Errorf("fetch account balances: %w", err)
	}

	computed := response.BalanceResponse.Computed
	if err := utils.ValidateFields(computed, "balance."); err != nil {
		return models.Balances{}, err
	}

	return models.Balances{
		CashBalance: *computed.NetCash,
		BuyingPower: *computed.BuyingPower,
	}, nil
}

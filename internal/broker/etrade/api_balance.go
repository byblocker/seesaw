package etrade

import (
	"context"
	"fmt"
	"net/url"
	"seesaw/internal/broker"
	"seesaw/internal/validate"
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

func (client *Client) fetchBalances(ctx context.Context, account Account) (models.Balances, error) {
	endpoint := client.endpointFor(account, "balance")
	query := url.Values{
		"instType":    {brokerageAccountType},
		"realTimeNAV": {"true"},
	}

	response, err := broker.FetchJSON[balanceResponse](ctx, client.httpClient, endpoint+"?"+query.Encode())
	if err != nil {
		return models.Balances{}, fmt.Errorf("fetch account balances: %w", err)
	}

	computed := response.BalanceResponse.Computed
	if err := validate.Fields(computed, "balance."); err != nil {
		return models.Balances{}, err
	}

	return models.Balances{
		CashBalance: *computed.NetCash,
		BuyingPower: *computed.BuyingPower,
	}, nil
}

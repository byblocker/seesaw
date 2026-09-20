package etrade

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"seesaw/internal/collections"
	"seesaw/internal/httpclient"
	"slices"
)

const (
	activeAccountStatus  = "ACTIVE"
	brokerageAccountType = "BROKERAGE"
)

type AccountDetails struct {
	Id     string `json:"accountId"`
	Key    string `json:"accountIdKey"`
	Type   string `json:"institutionType"`
	Status string `json:"accountStatus"`
}

type listAccountsAPIResponse struct {
	AccountListResponse struct {
		Accounts struct {
			AccountList []AccountDetails `json:"Account"`
		} `json:"Accounts"`
	} `json:"AccountListResponse"`
}

func (client *Client) GetAccounts(ctx context.Context) ([]AccountDetails, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	log.Print("Loading active brokerage accounts...")

	endpoint := fmt.Sprintf("%s/accounts/list", client.apiBaseURL)
	response, err := httpclient.SendJSONRequest[listAccountsAPIResponse](
		ctx, client.httpClient, http.MethodGet, endpoint, nil, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("could not load accounts: %w", err)
	}

	isSupportedAccount := func(account AccountDetails) bool {
		return account.Type == brokerageAccountType && account.Status == activeAccountStatus
	}
	accounts := collections.Filter(
		slices.Values(response.AccountListResponse.Accounts.AccountList),
		isSupportedAccount,
	)

	return slices.Collect(accounts), nil
}

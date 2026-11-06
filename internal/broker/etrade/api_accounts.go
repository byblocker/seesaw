package etrade

import (
	"context"
	"fmt"
	"seesaw/internal/broker"
	"seesaw/internal/validate"
)

const (
	activeAccountStatus  = "ACTIVE"
	brokerageAccountType = "BROKERAGE"
)

type Account struct {
	id  string
	key string
}

type accountResponse struct {
	Id     string `json:"accountId" validate:"min=4"`
	Key    string `json:"accountIdKey" validate:"required"`
	Type   string `json:"institutionType"`
	Status string `json:"accountStatus"`
}

type listAccountsAPIResponse struct {
	AccountListResponse struct {
		Accounts struct {
			AccountList []accountResponse `json:"Account"`
		} `json:"Accounts"`
	} `json:"AccountListResponse"`
}

func (client *Client) fetchAccounts(ctx context.Context) ([]Account, error) {
	endpoint := fmt.Sprintf("%s/accounts/list", client.apiBaseURL)
	stop := broker.StartTimer("Fetching brokerage accounts")
	response, err := broker.FetchJSON[listAccountsAPIResponse](
		ctx, client.httpClient, endpoint,
	)
	stop(err)
	if err != nil {
		return nil, fmt.Errorf("could not load accounts: %w", err)
	}

	accounts := make([]Account, 0, len(response.AccountListResponse.Accounts.AccountList))
	for _, account := range response.AccountListResponse.Accounts.AccountList {
		if account.Type != brokerageAccountType || account.Status != activeAccountStatus {
			continue
		}
		if err := validate.Fields(account, "account."); err != nil {
			return nil, err
		}
		accounts = append(accounts, Account{id: account.Id, key: account.Key})
	}
	return accounts, nil
}

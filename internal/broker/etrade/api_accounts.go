package etrade

import (
	"context"
	"fmt"
	"seesaw/internal/utils"
)

const (
	activeAccountStatus  = "ACTIVE"
	brokerageAccountType = "BROKERAGE"
)

type Account struct {
	id  string
	key string
}

func (account Account) ID() string { return account.id }

func (account Account) maskedID() string {
	return account.id[len(account.id)-4:]
}

type accountResponse struct {
	Id     string `json:"accountId"`
	Key    string `json:"accountIdKey"`
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

func (client *Client) Accounts(ctx context.Context) ([]Account, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/accounts/list", client.apiBaseURL)
	var timer utils.Timer
	timer.Start("Fetching brokerage accounts")
	response, err := utils.FetchJSON[listAccountsAPIResponse](
		ctx, client.httpClient, endpoint,
	)
	timer.Stop(err)
	if err != nil {
		return nil, fmt.Errorf("could not load accounts: %w", err)
	}

	accounts := make([]Account, 0, len(response.AccountListResponse.Accounts.AccountList))
	for _, account := range response.AccountListResponse.Accounts.AccountList {
		if account.Type != brokerageAccountType || account.Status != activeAccountStatus {
			continue
		}
		accounts = append(accounts, Account{id: account.Id, key: account.Key})
	}
	return accounts, nil
}

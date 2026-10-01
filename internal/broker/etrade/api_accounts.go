package etrade

import (
	"context"
	"fmt"
	"log"
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

	log.Print("Loading active brokerage accounts...")

	endpoint := fmt.Sprintf("%s/accounts/list", client.apiBaseURL)
	response, err := utils.FetchJSON[listAccountsAPIResponse](
		ctx, client.httpClient, endpoint,
	)
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

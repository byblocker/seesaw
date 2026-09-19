package etrade

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"seesaw/internal/httpclient"
)

const (
	activeAccountStatus  = "ACTIVE"
	brokerageAccountType = "BROKERAGE"
)

type accountDetails struct {
	Id     string `json:"accountId"`
	Key    string `json:"accountIdKey"`
	Type   string `json:"institutionType"`
	Status string `json:"accountStatus"`
}

type listAccountsAPIResponse struct {
	AccountListResponse struct {
		Accounts struct {
			AccountList []accountDetails `json:"Account"`
		} `json:"Accounts"`
	} `json:"AccountListResponse"`
}

func (client *Client) getAccountDetails(ctx context.Context, accountId string) (*accountDetails, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if client.account != nil && client.account.Id == accountId {
		account := *client.account
		return &account, nil
	}

	log.Printf("Finding brokerage account with ID: %v", accountId)

	endpoint := fmt.Sprintf("%s/accounts/list", client.apiBaseURL)
	response, err := httpclient.SendJSONRequest[listAccountsAPIResponse](
		ctx, client.httpClient, http.MethodGet, endpoint, nil, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("could not load accounts: %w", err)
	}

	accounts := response.AccountListResponse.Accounts.AccountList
	for i := range accounts {
		account := &accounts[i]

		isBrokerage := account.Type == brokerageAccountType
		isActive := account.Status == activeAccountStatus
		isTargetAccount := account.Id == accountId
		validAccountKey := account.Key != ""

		if isBrokerage && isActive && isTargetAccount && validAccountKey {
			log.Print("Found target brokerage account!")
			cached := *account
			client.account = &cached
			return account, nil
		}
	}

	return nil, fmt.Errorf("could not find brokerage account")
}

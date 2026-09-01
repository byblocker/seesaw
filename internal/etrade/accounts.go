package etrade

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"seesaw/internal"
)

const (
	ActiveAccountStatus  = "ACTIVE"
	BrokerageAccountType = "BROKERAGE"
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

func (client Client) GetAccountDetails(ctx context.Context, accountId string) (*AccountDetails, error) {
	log.Printf("Finding brokerage account with ID: %v", accountId)

	endpoint := fmt.Sprintf("%s/accounts/list", client.APIBaseUrl)
	response, err := internal.SendJSONRequest[listAccountsAPIResponse](
		ctx, client.httpClient, http.MethodGet, endpoint, nil, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("could not load accounts: %w", err)
	}
	accounts := response.AccountListResponse.Accounts.AccountList

	for i := range accounts {
		account := &accounts[i]

		isBrokerage := account.Type == BrokerageAccountType
		isActive := account.Status == ActiveAccountStatus
		isTargetAccount := account.Id == accountId
		validAccountKey := account.Key != ""

		if isBrokerage && isActive && isTargetAccount && validAccountKey {
			log.Print("Found target brokerage account key!")
			return account, nil
		}
	}

	return nil, fmt.Errorf("could not find brokerage account")
}

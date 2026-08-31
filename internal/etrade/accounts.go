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

type etradeAccountDTO struct {
	Id              string `json:"accountId"`
	Key             string `json:"accountIdKey"`
	InstitutionType string `json:"institutionType"`
	Status          string `json:"accountStatus"`
}

type listAccountsAPIResponse struct {
	AccountListResponse struct {
		Accounts struct {
			AccountList []etradeAccountDTO `json:"Account"`
		} `json:"Accounts"`
	} `json:"AccountListResponse"`
}

func (client Client) GetAccountKey(ctx context.Context, targetAccountId string) (string, error) {
	log.Printf("Finding brokerage account with ID: %v", targetAccountId)

	endpoint := fmt.Sprintf("%s/accounts/list", client.APIBaseUrl)
	response, err := internal.SendJSONRequest[listAccountsAPIResponse](
		ctx, client.Client, http.MethodGet, endpoint, nil, nil,
	)
	if err != nil {
		return "", fmt.Errorf("could not load accounts: %w", err)
	}
	accounts := response.AccountListResponse.Accounts.AccountList

	for i := range accounts {
		account := &accounts[i]

		isBrokerage := account.InstitutionType == BrokerageAccountType
		isActive := account.Status == ActiveAccountStatus
		isTargetAccount := account.Id == targetAccountId
		validAccountKey := account.Key != ""

		if isBrokerage && isActive && isTargetAccount && validAccountKey {
			log.Print("Found target brokerage account key!")
			return account.Key, nil
		}
	}

	return "", fmt.Errorf("could not find brokerage account")
}

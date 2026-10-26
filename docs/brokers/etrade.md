# ETRADE

[Back to the Seesaw README](../../README.md)

## Setup

1. Obtain an API key and secret: [ETRADE getting started and requesting keys](https://developer.etrade.com/getting-started).

4. Run from the repository root in an interactive terminal:

```sh
go run . connect --broker ETRADE
```

Enter the API base URL, API key, and API secret when prompted.

The default base endpoint is `https://api.etrade.com/v1`. Press `ENTER` to use, or provide the QA/sandbox path otherwise.

Seesaw will validate, save this configuration in your OS keyring, and then open the OAuth sign-in flow. Enter your verification from ETRADE when prompted.

## Creating a portfolio plan

Use `broker: ETRADE` and your ETRADE account number as `accountId` in your portfolio plan. See the [portfolio example](../../README.md#creating-your-portfolio-plan).

## Replacing configuration

Re-run `connect` to replace credentials or change the endpoint. If a configuration currently exits, _seesaw_ will ask before overwriting it. Changing credentials requires a new OAuth sign-in.

If authentication fails after saving, run `seesaw snapshot <portfolio>` to retry sign-in with the saved configuration, or re-run `connect` to replace it.

## Authentication

ETRADE expires OAuth tokens at midnight in the `America/New_York` timezone. _seesaw_ reuses cached tokens during the day and renews them after two hours of inactivity. 

When _seesaw_ first authenticates for the day, _seesaw_ will open the browser to start the authorization flow.

## Limitations

- ETRADE purchases use whole shares; the client settings reports `AllowFractionalShares: false`. ETRADE is actively supporting fractional shares but only for specific tickers.
- Transaction history covers active accounts accessible through your ETRADE credentials. _seesaw_ cannot account for activity at other brokerages or in closed or inactive ETRADE accounts.
- Portfolio requests follow the broker's pagination metadata to retrieve all holdings. Missing continuation metadata or repeated positions cause an error instead of returning an incomplete portfolio.

## Library usage

Supply a `broker.ETradeConfig` and connect via `broker.ConnectETrade(ctx, config)` from `seesaw/pkg/broker` to receive a `*broker.ETradeClient`, which implements `models.BrokerClient`. Populate the configuration from your own environment loader or secrets manager, as needed. _seesaw_ still uses OS keyring for OAuth token caching.

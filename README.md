# Seesaw

_seesaw_ automates portfolio rebalancing and tax-loss harvesting. Plan assets with target allocations and tax-loss-harvesting ticker pairs, and then, run `rebalance` to inspect any actions suggested for your portfolio.

Currently, _seesaw_ is purely diagnostic; actions are listed in YAML format and not automatically submitted.

## Setup

1. Install Go 1.26 or later: [Go installation guide](https://go.dev/doc/install).

2. Download the project dependencies
```sh
go mod download
```

4. Secrets and OAuth token caching relies upon your OS keyring. Verify access: [go-keyring platform support and requirements](https://github.com/zalando/go-keyring#readme).

5. Broker authorization requires an internet connection and the ability to open a browser from your terminal. To check browser access, run the command for your OS:

```sh
# macOS
open https://example.com

# Linux desktop
xdg-open https://example.com

# Windows PowerShell
Start-Process https://example.com
```

6. Connect your broker using its setup guide:
- [ETRADE setup](docs/brokers/etrade.md)

## Creating your portfolio plan

1. Create a personal copy of the template portfolio plan:
```sh
cp data/portfolios/template.yaml data/portfolios/< your portfolio name here >.yaml
```

**NOTE**: Personal portfolio .yaml files are ignored through `.gitignore`; only `data/portfolios/template.yaml` is tracked for explanation purposes.

2. Edit your local portfolio plan:

```yaml
broker: YOUR_BROKER_NAME
accountId: "YOUR_ACCOUNT_ID"

settings:
  min_fractional_share_to_round: 0.7
  max_pct_alloc_deviation_allowed: 0.02
  min_pct_loss_to_harvest: -0.005

assets:
  - primary_ticker: VOO
    alternate_ticker: SCHX
    target_allocation: 0.75

  - primary_ticker: SGOV
    target_allocation: 0.25
```

`broker` and `accountId` should be your broker name and brokerage account number.

Rebalancing settings are defined in the `settings` struct:
- `min_fractional_share_to_round` is the remainder needed to round a whole-share buy up, between `0` and `1` inclusive. Fractional-share support depends on the broker. See its setup guide for supported behavior.
- `max_pct_alloc_deviation_allowed` is extra whole-share purchase allowance as a fraction of total portfolio value, between `0` and `1` inclusive, limited by available cash.
- `min_pct_loss_to_harvest` is the loss threshold, between `-1` and `0` inclusive. 

There are no defaults. Missing settings are rejected, while zero values are accepted. 

For each `asset`, you must choose a . . .
- `primary_ticker` and `alternate_ticker`: These identify publically-tradable security pairs suitable for tax-loss harvesting. The primary is preferred for most purchases, but the alternate is used when harvesting losses from primary, and vice versa. Without an `alternate_ticker`, _seesaw_ will propose buys without suggesting tax-loss harvesting.

**WARNING:** _seesaw_ does not ensure the pairs tickers suitability for wash-sale avoidance. Please review this yourself.

- `target_allocation`: The target fraction of your portfolio dedicated to this pair of securities. Both the primary and alternate position allocation count towards this total. Each allocation must be greater than `0.0` and at most `1.0`. _seesaw_ ensures the sum of all `target_allocations` equals `1.0`

## Rebalance!

1. Pass your portfolio filename `data/portfolios/` without `.yaml`. _seesaw_ reads files relative to your working directory.

```sh
go run . rebalance --portfolio < name of your portfolio >
```

2. _seesaw_ will create a output of a list of `buys`with suggested quantities and estimated costs, and `sells`, with specific lots, estimated proceeds, and the profit/loss realized:

```yaml
buys:
    - ticker: SGOV
      price: "100.00"
      quantity: "1"
      estimatedcost: "100"
sells:
    - ticker: VOO
      type: MARKET
      price: "100"
      lots:
        - lot:
            id: 123456789
            quantity: "100"
            pricepaid: "90"
            timeacquired: 2026-01-01T00:00:00-04:00
            marketvalue: "10000"
            unrealizedpl: "1000"
            plpct: "0.10"
          quantity: "100"
```

### Rebalance Behavior

`rebalance` fetches a snapshot of your account and then . . .
1. Assigns positions to `Assets` according to the provided portfolio plan;
2. Identifies lots with tax loss and simulates sales, if any;
3. Sizes purchases using available cash balance and proceeds from suggested sells

For any holdings outside of the planned positions, _seesaw_ will attempt to harvest any tax-loss; it does not force liquidation to reduce tax liability. Similarly, _seesaw_ will not force liquidation of overweight assets to reduce allocation.

_seesaw_ uses recent purchases from the snapshot to help avoid wash sales; however, the available transaction history is limited to a single broker. Review its setup guide for coverage and limitations. Avoid wash-sales by ensuring different portfolios use different securities.

## Library Usage

Use `pkg.Connect(ctx, name)` with a saved configuration in your OS keyring, or see specific usage for a given broker:
- [ETRADE guide](docs/brokers/etrade.md#library-use)

## Development Choices

We use `shopspring/decimal` for decimal arithmetic and JSON numbers such as `2.0E-4`. We keep prices and quantities as decimals throughout the calculations and truncate division to 19 decimal places.

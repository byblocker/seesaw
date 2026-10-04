# Seesaw

_seesaw_ automates portfolio rebalancing and tax-loss harvesting. Plan assets with target allocations and tax-loss-harvesting ticker pairs, and then, run `rebalance` to inspect any actions suggested for your portfolio.

_seesaw_ displays portfolio allocations and suggested orders in tables. It does not submit orders.

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
is_taxable: true

rebalance:
  min_fractional_share_to_round: 0.7
  max_pct_allocation_deviation: 0.02
  min_pct_loss_to_harvest: -0.005

liquidation:
  lot_selection: estimated_tax
  max_pct_allocation_deviation: 0.02
  cash_reserve: 0
  max_excess_cash: 100

assets:
  - primary_ticker: VOO
    alternate_ticker: SCHX
    target_allocation: 0.75

  - primary_ticker: SGOV
    target_allocation: 0.25
```

All fields below are required except `assets[].alternate_ticker`. Values have no defaults or user-config overrides; explicit zeros are accepted where allowed.

| Field | Purpose | Values / constraints |
| --- | --- | --- |
| `broker` | Name of a supported brokerage | `ETRADE` |
| `accountId` | Brokerage account number. | String |
| `is_taxable` | Is the account taxable? | `true` or `false`. |
| `rebalance.min_fractional_share_to_round` | Remainder needed to round a fractional share to the next whole number. Fractional-share support varies by broker. |  `[0, 1]` |
| `rebalance.max_pct_allocation_deviation` | If rounding up, how high may an asset deviate from its target allocation (limited by buying power) | `[0, 1]` |
| `rebalance.min_pct_loss_to_harvest` | Threshold a lot's loss must meet to qualify for tax-loss harvesting | `[-1, 0]` |
| `liquidation.lot_selection` | Select lots by estimated tax cost or a fixed priority: short-term losses, long-term losses, long-term gains, then short-term gains. | `estimated_tax` or  `min_tax_order` |
| `liquidation.max_pct_allocation_deviation` | If liquidating, how low may an asset deviate from its target deviation | `[0, 1]` |
| `liquidation.cash_reserve` | Minimum balance for this account | Non-negative number |
| `liquidation.max_excess_cash` | Maximum balance above requested liquidation amount |  Non-negative number |
| `assets[].primary_ticker` | Preferred ticker for purchases. Its holdings and the alternate's holdings count towards the same asset allocation. | Uppercase and unique across all asset tickers. |
| `assets[].alternate_ticker` | A suitable partner for tax-loss harvesting. When harvesting one ticker, _seesaw_ can purchase the other to maintain the asset's allocation. Without an alternate, it can propose buys without paired tax-loss harvesting. | Optional; uppercase and unique across all asset tickers. |
| `assets[].target_allocation` | The target fraction of the portfolio dedicated to the primary and alternate securities together. _seesaw_ ensures all target allocations sum to `1.0`. | `[0, 1]` |

_seesaw_ rejects unknown fields and multiple documents

**WARNING:** _seesaw_ does not ensure ticker pairs are suitable for wash-sale avoidance. Please review this yourself.

## Rebalance!

1. Pass your portfolio filename `data/portfolios/` without `.yaml`. _seesaw_ reads files relative to your working directory.

```sh
go run . rebalance --portfolio < name of your portfolio >
```

2. Review the account snapshot: asset values, current and target allocations, cash balance, and portfolio total.

3. Review the suggested orders. For example, with a starting cash balance of $1,000:

```text
Suggested Orders

Ticker  Action  Acquired    Shares    Price        Cost       P/L
_________________________________________________________________
VOO     SELL    2026-06-10      10  $100.00   $1,000.00  -$100.00

SCHX    BUY     —               15  $100.00  -$1,500.00         —

SGOV    BUY     —                5  $100.00    -$500.00         —
_________________________________________________________________

Total P/L:         -$100.00
Remaining balance:    $0.00
```

When no trades qualify, you will see `No trades suggested.`

Rebalance logs show quote fetching, tax-loss evaluation, purchase sizing, and final trade counts. Add `--verbose` (or `-v`) to show source locations, individual transaction page results, and reasons for skipped trades:

```sh
go run . rebalance --portfolio < name of your portfolio > --verbose
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

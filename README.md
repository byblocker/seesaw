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

1. Create a personal copy of the example portfolio plan:
```sh
mkdir -p data/portfolios
cp examples/portfolio.yaml data/portfolios/< your portfolio name here >.yaml
```

**NOTE**: `data/` holds your personal files and is ignored through `.gitignore`. The tracked examples live in `examples/`.

2. Edit your local portfolio plan:

```yaml
broker: YOUR_BROKER_NAME
accountId: "YOUR_ACCOUNT_ID"
is_taxable: true

assets:
  - primary_ticker: TICKER_A
    alternate_ticker: TICKER_B
    target_allocation: 0.75

  - primary_ticker: TICKER_C
    target_allocation: 0.25
```

All fields below are required except `assets[].alternate_ticker`.

| Field | Purpose | Values / constraints |
| --- | --- | --- |
| `broker` | Name of a supported brokerage | `ETRADE` |
| `accountId` | Brokerage account number. | String |
| `is_taxable` | Is the account taxable? | `true` or `false`. |
| `assets[].primary_ticker` | Preferred ticker for purchases. Its holdings and the alternate's holdings count towards the same asset allocation. | Uppercase and unique across all asset tickers. |
| `assets[].alternate_ticker` | A suitable partner for tax-loss harvesting. When harvesting one ticker, _seesaw_ can purchase the other to maintain the asset's allocation. Without an alternate, it can propose buys without paired tax-loss harvesting. | Optional; uppercase and unique across all asset tickers. |
| `assets[].target_allocation` | The target fraction of the portfolio dedicated to the primary and alternate securities together. _seesaw_ ensures all target allocations sum to `1.0`. | `[0, 1]` |

_seesaw_ rejects unknown fields and multiple documents

**WARNING:** _seesaw_ does not ensure ticker pairs are suitable for wash-sale avoidance. Please review this yourself.

## User settings

1. Create a personal copy of the example user config:
```sh
mkdir -p data
cp examples/user.yaml data/user.yaml
```

2. Edit your local user config:

```yaml
tax:
  short_term_gain_rate: 0.35
  long_term_gain_rate: 0.20

rebalance:
  min_fractional_share_to_round: 0.7
  max_pct_allocation_deviation: 0.02
  min_pct_loss_to_harvest: -0.005

liquidation:
  max_pct_allocation_deviation: 0.02
  max_excess_cash: 100
```

| Field | Purpose | Values / constraints |
| --- | --- | --- |
| `tax.short_term_gain_rate` | Combined marginal tax rate used to estimate tax on short-term gains, including federal, state, and any additional taxes. | `[0, 1]` |
| `tax.long_term_gain_rate` | Combined marginal tax rate used to estimate tax on long-term gains, including federal, state, and any additional taxes. | `[0, 1]` |
| `rebalance.min_fractional_share_to_round` | Remainder needed to round a fractional share to the next whole number. Fractional-share support varies by broker. |  `[0, 1]` |
| `rebalance.max_pct_allocation_deviation` | If rounding up, how high may an asset deviate from its target allocation (limited by buying power) | `[0, 1]` |
| `rebalance.min_pct_loss_to_harvest` | Threshold a lot's loss must meet to qualify for tax-loss harvesting | `[-1, 0]` |
| `liquidation.max_pct_allocation_deviation` | How far an asset may fall below its target after a withdrawal. For example, `0.02` lets a `0.25` target fall to `0.23`. | `[0, 1]` |
| `liquidation.max_excess_cash` | Maximum sale proceeds above the requested amount. Whole-share sales rarely land on the exact amount; a larger value gives the planner more room to lower estimated tax. | Non-negative number |

These settings apply to every portfolio; tax rates are used only for taxable ones. All fields are required and have no defaults; explicit zeros are accepted where allowed.

`rebalance` and `liquidate` load `data/user.yaml` before connecting to the broker.

Each proposed sale will show it's estimated tax impact. Negative impact represents tax relief, assuming losses offset gains of the same holding period. These estimates do not account for annual netting, carryovers, or wash-sale adjustments. These are not a calculation of tax due.

Use `config.ParseUser(reader)` from `seesaw/pkg/config` to parse and validate. It rejects unknown fields and additional YAML documents.

## Rebalance!

1. Pass your portfolio filename `data/portfolios/` without `.yaml`. _seesaw_ reads files relative to your working directory.

```sh
go run . rebalance --portfolio < name of your portfolio >
```

2. Review the account snapshot: asset values, current and target allocations, cash balance, and portfolio total.

3. Review the suggested orders. For example, with a starting cash balance of $1,000:

```text
Suggested Orders

Ticker    Action  Acquired    Shares    Price        Cost       P/L
___________________________________________________________________
TICKER_A  SELL    2026-06-10      10  $100.00   $1,000.00  -$100.00

TICKER_B  BUY     —               15  $100.00  -$1,500.00         —

TICKER_C  BUY     —                5  $100.00    -$500.00         —
___________________________________________________________________

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

## Liquidate!

Request a withdrawal in your account's currency:

```sh
go run . liquidate --portfolio growth --amount 1000
```

_seesaw_ uses existing cash first and proposes whole-share sales for any shortfall. Fractional holdings still count toward allocation, but are not sold.

Sale proceeds stay between the amount you need and that amount plus `liquidation.max_excess_cash`. Within that range, the planner chooses sales in this order of priority:

1. Keep every asset at or above its minimum allocation. When that is impossible, fall short by as few dollars as possible.
2. For taxable portfolios, owe the least estimated tax. Losses offset gains in the same sale, but the planner does not sell extra loss lots only to harvest them.
3. Raise the amount closest to your request.

Remaining ties prefer holdings outside your plan, then lots with the lowest estimated tax per dollar. Allocations are measured against the portfolio after the requested withdrawal, so the planner never sells more than needed only to rebalance.

Review the proposed lots and the totals: cash on hand plus sale proceeds, minus the withdrawal, gives the cash left over. Taxable portfolios also show the net estimated tax, where a negative amount is potential relief. If any asset ends up outside its allocation band, _seesaw_ lists it with its resulting allocation. A `[WS]` tag identifies loss sales with recent purchases of the same ticker. Cash availability may depend on settlement or broker restrictions. Tax estimates do not reserve cash for tax payments.

The search stops with an error if your whole-share holdings cannot raise the amount needed, if no combination of whole shares lands within `liquidation.max_excess_cash` of it, or if its search budget is exhausted. It does not return an unfinished proposal.

Typical requests finish in a few thousand nodes, well under a second. The default budget of 100,000 nodes leaves wide headroom while keeping a hard request from running for long. Rarely, a large request across many cheap and expensive shares needs more; raise the budget for that request:

```sh
go run . liquidate --portfolio growth --amount 60000 --search-limit 5000000
```

Like `rebalance`, this command only displays suggestions; it does not submit orders or withdraw cash.

## Library Usage

Use `broker.Connect(ctx, name)` from with a saved configuration in your OS keyring, or see specific usage for a given broker:
- [ETRADE guide](docs/brokers/etrade.md#library-use)

## Development Choices

We use `shopspring/decimal` for decimal arithmetic and JSON numbers such as `2.0E-4`. We keep prices and quantities as decimals throughout the calculations and truncate division to 19 decimal places.

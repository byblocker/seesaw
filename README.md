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

1. Create your user config and a portfolio plan from the examples:
```sh
go run . init < your portfolio name here >
```

This writes `data/user.yaml`, unless it already exists, and `data/portfolios/< your portfolio name here >.yaml`. Rerun `init` with another name to add more plans.

**NOTE**: `data/` holds your personal files and is ignored through `.gitignore`. The tracked examples live in `examples/`.

2. Edit your local portfolio plan:

```yaml
broker: YOUR_BROKER_NAME
accountId: "YOUR_ACCOUNT_ID"
is_taxable: true

settings:
  min_fractional_share_to_round: 0.7
  min_pct_loss_to_harvest: -0.005
  allocation_upper_limit: 0.02
  allocation_lower_limit: 0.02

assets:
  - primary_ticker: TICKER_A
    alternate_ticker: TICKER_B
    target_allocation: 0.75

  - primary_ticker: TICKER_C
    target_allocation: 0.25
```

All fields below are required except `assets[].alternate_ticker`. None have defaults; explicit zeros are accepted where allowed.

| Field | Purpose | Values / constraints |
| --- | --- | --- |
| `broker` | Name of a supported brokerage | `ETRADE` |
| `accountId` | Brokerage account number. | String |
| `is_taxable` | Is the account taxable? | `true` or `false`. |
| `settings.min_fractional_share_to_round` | Remainder needed to round a fractional share to the next whole number. Fractional-share support varies by broker. |  `[0, 1]` |
| `settings.min_pct_loss_to_harvest` | Threshold a lot's loss must meet to qualify for tax-loss harvesting | `[-1, 0]` |
| `settings.allocation_upper_limit` | How far `rebalance` may lift an asset above its target when rounding a buy up to a whole share, measured against the portfolio before trading and limited by buying power. With `rebalance --force`, also how far an asset may drift above its target before it is sold back down. | `[0, 1]` |
| `settings.allocation_lower_limit` | How far `liquidate` may sell an asset below its target, measured against the portfolio after the withdrawal. For example, `0.02` lets a `0.25` target fall to `0.23`. | `[0, 1]` |
| `assets[].primary_ticker` | Preferred ticker for purchases. Its holdings and the alternate's holdings count towards the same asset allocation. | Uppercase and unique across all asset tickers. |
| `assets[].alternate_ticker` | A suitable partner for tax-loss harvesting. When harvesting one ticker, _seesaw_ can purchase the other to maintain the asset's allocation. Without an alternate, it can propose buys without paired tax-loss harvesting. | Optional; uppercase and unique across all asset tickers. |
| `assets[].target_allocation` | The target fraction of the portfolio dedicated to the primary and alternate securities together. _seesaw_ ensures all target allocations sum to `1.0`. | `[0, 1]` |

_seesaw_ rejects unknown fields and multiple documents

**WARNING:** _seesaw_ does not ensure ticker pairs are suitable for wash-sale avoidance. Please review this yourself.

## User settings

Edit your local user config, `data/user.yaml`, created by `init`:

```yaml
tax:
  short_term_gain_rate: 0.35
  long_term_gain_rate: 0.20
```

| Field | Purpose | Values / constraints |
| --- | --- | --- |
| `tax.short_term_gain_rate` | Combined marginal tax rate used to estimate tax on short-term gains, including federal, state, and any additional taxes. | `[0, 1]` |
| `tax.long_term_gain_rate` | Combined marginal tax rate used to estimate tax on long-term gains, including federal, state, and any additional taxes. | `[0, 1]` |

These rates apply to every portfolio. They are used only for taxable portfolios, so you can leave out the `tax` section, leaving the file empty, if none of yours are taxable; _seesaw_ stops before connecting to the broker if a taxable portfolio has no rates.

`snapshot`, `rebalance`, and `liquidate` load `data/user.yaml` before connecting to the broker.

Each proposed sale will show it's estimated tax impact. Negative impact represents tax relief, assuming losses offset gains of the same holding period. These estimates do not account for annual netting, carryovers, or wash-sale adjustments. These are not a calculation of tax due.

_seesaw_ rejects unknown fields and multiple documents.

## Snapshot

See where your account stands without planning any trades:

```sh
go run . snapshot < name of your portfolio >
```

A snapshot fetches your holdings and recent transactions, plus a quote for every ticker in your plan and every other ticker you hold.

`rebalance` and `liquidate` plan from it.

## Rebalance!

1. Pass the name of your portfolio plan in `data/portfolios/`, without `.yaml`. _seesaw_ reads files relative to your working directory.

```sh
go run . rebalance < name of your portfolio >
```

2. Review the account snapshot: asset values at the bid, current and target allocations, cash balance, and portfolio total.

3. Review the suggested orders. For example, with a starting cash balance of $1,000:

```text
Suggested Orders

Ticker    Action  Acquired    Shares    Price        Cost       P/L
───────────────────────────────────────────────────────────────────
TICKER_A  SELL    2026-06-10      10  $100.00   $1,000.00  -$100.00
TICKER_B  BUY                     15  $100.00  -$1,500.00
TICKER_C  BUY                      5  $100.00    -$500.00
```

Taxable portfolios add a Tax column, and below the orders, the realized short- and long-term P/L and their total with the net estimated tax, where a negative amount is potential relief.

When no trades qualify, you will see `No trades suggested.`, and the Estimated Portfolio matches the snapshot.

4. Review the Estimated Portfolio: each asset's value, allocation, and deviation from target after the suggested orders, with buys counted at their estimated cost. The Cash row is the cash left in the account, ready to withdraw or transfer, and the Total includes it.

Add `--verbose` (`-v`) to any command to see its progress logs. For `rebalance`, they show quote fetching for the snapshot, each asset's tax-loss and purchase decisions with the reasons for skipped trades, purchase sizing, and final trade counts.

### Rebalance Behavior

`rebalance` fetches a snapshot of your account and then . . .
1. Assigns positions to `Assets` according to the provided portfolio plan;
2. Identifies lots with tax loss and simulates sales, if any, in taxable portfolios only;
3. Sizes purchases using available cash balance and proceeds from suggested sells

By default, _seesaw_ only harvests tax losses from holdings outside the plan, and it does not sell overweight assets to reduce their allocation. Use `--force` to sell both.

Portfolios with `is_taxable: false` skip tax-loss harvesting: a loss there brings no tax relief, and buying the replacement ticker could disallow a loss in one of your taxable accounts as a wash sale.

_seesaw_ uses recent purchases from the snapshot to help avoid wash sales; however, the available transaction history is limited to a single broker. Review its setup guide for coverage and limitations. Avoid wash-sales by ensuring different portfolios use different securities.

### Forced Rebalance

```sh
go run . rebalance < name of your portfolio > --force
```

With `--force` (`-f`), _seesaw_ also . . .
1. Sells each planned asset more than `settings.allocation_upper_limit` above its target down to its target, in whole shares, so it may stay up to one share above;
2. Sells every whole share of holdings outside the plan;
3. Adds the proceeds to the cash used to size purchases for underweight assets. Proceeds beyond what those purchases need stay as cash.

It sells the lots with the lowest estimated tax per dollar raised first, so taxable portfolios sell the deepest losses first and the smallest gains last; without tax rates, lots sell in lot order. Fractional shares are not sold. Review each sale's estimated tax in taxable portfolios: `--force` suits tax-advantaged accounts best.

## Liquidate!

Request a withdrawal in your account's currency:

```sh
go run . liquidate growth --amount 1000
```

`-a` is short for `--amount`. Either accepts a leading `$` and commas between thousands, such as `'$1,000.50'`. Quote amounts that start with `$` so your shell does not expand them.

_seesaw_ uses existing cash first and proposes whole-share sales for any shortfall. Fractional holdings still count toward allocation, but are not sold.

The planner sells holdings outside your plan first. Within each group, it ranks lots by their estimated tax per dollar raised, so lots compare fairly across share prices, and sells the lowest first. For taxable portfolios, that sells the deepest losses first and the smallest gains last, weighing each lot at its short- or long-term rate. Ties go to the asset furthest above its target, then follow your plan's asset order.

Each planned asset sells only down to its minimum allocation, never below it; when its next whole share would cross the minimum, the planner sells from other assets instead. Allocations are measured against the portfolio after the requested withdrawal, so the planner never sells more than needed only to rebalance. The last lot sold rounds up to a whole share, so proceeds can exceed the amount needed by less than one share.

Each withdrawal is planned on its own at your configured rates. The planner does not net gains and losses across the tax year, apply loss carryforwards, or account for tax brackets.

Review the proposed lots; for taxable portfolios, the table ends with the realized short- and long-term P/L and the net estimated tax, where a negative amount is potential relief. Then review the Estimated Portfolio: each asset's value, allocation, and deviation from target after the sale and the withdrawal. Its Cash row is the cash left in the account once the withdrawal leaves, and the Total includes it. A `[WS]` tag identifies loss sales with recent purchases of the same ticker. Cash availability may depend on settlement or broker restrictions. Tax estimates do not reserve cash for tax payments.

The planner stops with an error if your whole-share holdings cannot raise the amount needed, or if allocation minimums leave too little to sell; raise `settings.allocation_lower_limit` to allow more. It does not return an unfinished proposal.

Like `rebalance`, this command only displays suggestions; it does not submit orders or withdraw cash.

## Library Usage

The commands are one way to run _seesaw_; the packages under `seesaw/pkg` let another host, such as a scheduled job, run the same steps:

```go
ctx := context.Background()

plan, err := config.ParsePlan(planReader) // or config.LoadPlan(path)
user, err := config.ParseUser(userReader) // or config.LoadUser(path)
rates, err := user.TaxRates(plan.IsTaxable)

selected, err := registry.FindBroker(plan.Broker, store)
client, err := selected.Connect(ctx)
snapshot, err := portfolio.TakeSnapshot(ctx, client, plan, rates)

proposal, err := rebalancer.Plan(snapshot, plan.Settings, false) // or liquidation.PlanSales(snapshot, plan.Settings, amount)
estimated := snapshot.Simulate(proposal)
```

Check each `err` before continuing. The planners return a `portfolio.Proposal` of sells, buys, and the withdrawal, and `Simulate` returns the portfolio after it; render them however your host needs. Progress goes to the standard `log` package.

### Secrets

Brokers keep their credentials and tokens in a `secrets.Store` from `seesaw/pkg/secrets`: anything that can `Get` and `Set` bytes by key, returning `secrets.ErrNotFound` for a missing key. The commands use `keyring.Store{}` from `seesaw/pkg/secrets/keyring`; another host can back it with its own secret manager. `Connect` returns `models.ErrBrokerNotConfigured` when the store holds no credentials.

ETRADE signs in through a browser and expires tokens at midnight Eastern, so a host that cannot prompt can only reuse a token signed in that day and saved to its store.

### Building plans in code

`config.NewPlan(broker, accountId, isTaxable, settings, assets)` applies the same rules as a plan file: it trims tickers, rejects duplicates, and requires targets that sum to `1.0`. Set every `config.Settings` field; a field left out is zero, which is valid for each of them. `portfolio.TakeSnapshot` checks the tax rates, and the planners check their settings and amount, so values built in code get the same errors as the files. `ParsePlan` and `ParseUser` reject unknown fields and multiple documents.

## Development Choices

We use `shopspring/decimal` for decimal arithmetic and JSON numbers such as `2.0E-4`. We keep prices and quantities as decimals throughout the calculations and truncate division to 19 decimal places.

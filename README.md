# Seesaw

_seesaw_ plans portfolio rebalancing and tax-loss harvesting for ETRADE accounts. Define specific assets with target allocations and tax-loss-harvesting ticker pairs. Then, run the CLI to inspect any actions suggested for your portfolio.

Currently, _seesaw_ outputs an order plan in YAML. It does not automatically submit orders; _seesaw_ is purely diagnostic.

## Setup

1. Install Go 1.26 or later: [Go installation guide](https://go.dev/doc/install).
2. Fetch ETRADE API credentials: [ETRADE getting started and requesting keys](https://developer.etrade.com/getting-started).
3. Verify access to your OS keyring: [go-keyring platform support and requirements](https://github.com/zalando/go-keyring#readme).
4. Verify you can open a browser from your terminal for the OAuth sign-in flow: [ETRADE authorization documentation](https://apisb.etrade.com/docs/api/authorization/authorize.html).

5. Run these commands from the repository root:

```sh
go mod download
cp .env.template .env
mkdir -p data/portfolios
```

6. Fill in `.env` at the repository root with your credentials:

```dotenv
ETRADE_API_BASE_URL=https://api.etrade.com/v1
ETRADE_API_KEY=your-api-key
ETRADE_API_SECRET=your-api-secret
```

You can also supply these values through environment variables; they take precedence over `.env`. Use the proper credentials for the repsective BASE_URL; this production URL requires production credentials.

## Portfolio plan

Create `data/portfolios/default.yaml`:

```yaml
broker: ETRADE
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

Replace the **acountId** placeholder with your own ETRADE account number.

`broker` is required and currently supports only `ETRADE`.
Rebalancing thresholds are required in each portfolio's flat `settings` block:

- `min_fractional_share_to_round` is the remainder needed to round a whole-share buy up, between `0` and `1` inclusive.
- `max_pct_alloc_deviation_allowed` is extra whole-share purchase allowance as a fraction of total portfolio value, between `0` and `1` inclusive, limited by available cash.
- `min_pct_loss_to_harvest` is the loss threshold, between `-1` and `0` inclusive. The example's `-0.005` requires a loss of at least 0.5% before harvesting.

There are no implicit defaults. Missing or null settings are rejected before broker authentication; explicit zero values are accepted. Existing portfolio files must add all three fields. The values above preserve the previous behavior.

Fractional-share support is defined by each broker implementation. The ETRADE integration returns `false` from `CanBuyFractionalShares`, preserving whole-share purchases.

For each asset, you must choose a . . .
1. `primary_ticker` and `alternate_ticker`: These symbols for securities eligible for tax loss harvesting. You must provide the primary ticker, which is the preferred security for most purchases. You can specify an alternate ticker to buy when harvesting losses in the primary position; the primary position will then be used when harvesting from the alternate. Without an alternate, _seesaw_ will propose buys but will abstain from suggesting tax-loss harvesting orders.

**WARNING:** _seesaw_ does not verify that provided security pairs offer comparable market exposure nor determines whether the pair are substancitaly identical or not for wash-sale avoidance. You must review the pairs suitability yourself. 

2. `target_allocation`: The target fraction of your portfolio dedicated to these pair of securities. Both the primary and alternate positions combined count towards this total. Each allocation must be greater than `0.0` and at most `1.0`. _seesaw_ ensures the sum of all asset allocations equals `1.0`

## Run

Run from the repository root:

```sh
go run . rebalance --portfolio default
```

Pass the portfolio filename from `data/portfolios/` without `.yaml`. Seesaw reads `.env` and portfolio files relative to your working directory.

You must authenticate with ETRADE once per day, at least;  _seesaw_, however, reuses/refreshes these tokens for subsequent, same-day invocations (ETRADE routinely revokes OAuth tokens every night at midnight).

The output will contain a list of `buys`, with suggested quantities and estimated costs, and `sells`, with specific lots, estimated proceeds, and the profit/loss realized.

To build and run a binary from the repository root:

```sh
go build -o seesaw .
./seesaw rebalance --portfolio default
```

### Rebalance Behavior

`rebalance` fetches a snapshot of your account and then completes the following:
1. Assign portfolio positions to Assets according to the provided configuration
2. Proposes sales for lots with tax loss
3. Sizes purchases using available cash balance and proceeds from suggested sells

For holdings outside of the planned positions, _seesaw_ attempts to harvest any tax loss; it does not force liquidation to reduce tax liability. 

Similarly, _seesaw_ will not force liquidation of overweight assets to reduce allocation.

_seesaw_ prevents wash-sales by blocking a purchase after a recent sale, and it's sale after a recent purchase. This, however, is limited to accounts accessible in ETRADE. _seesaw_ cannot account for trading activity in other brokerages and activity in close/inactive ETRADE accounts.

ETRADE returns up to 50 positions per portfolio API call by default. _seesaw_ assumes no account would exceed this and does not paginate portfolio requests.

## Configuration

The CLI calls `pkg.ConnectFromENV(ctx, plan.Broker)`, which loads an optional `.env` file from the working directory before reading the selected broker's environment variables. Existing environment variables take precedence, including explicitly empty values. A missing `.env` file is allowed; other loading errors are returned. ETrade uses `ETRADE_API_BASE_URL`, `ETRADE_API_KEY`, and `ETRADE_API_SECRET`, with validation performed before authentication.

Library callers can use `pkg.ConnectFromENV` with their environment. ETrade configuration and validation live in `internal/broker/etrade.go`; `pkg.ETradeConfig` aliases the provider's configuration type.

Library callers can construct a `models.RebalanceSettings` value and pass it directly to `pkg.Rebalance`:

```go
roundingThreshold := decimal.RequireFromString("0.7")
allocationAllowance := decimal.RequireFromString("0.02")
harvestThreshold := decimal.RequireFromString("-0.005")
settings := models.RebalanceSettings{
    MinFractionalShareToRound:   &roundingThreshold,
    MaxPctAllocDeviationAllowed: &allocationAllowance,
    MinPctLossToHarvest:         &harvestThreshold,
}
orders, err := pkg.Rebalance(ctx, snapshot, client, settings)
```

Use `seesaw/pkg/models` and `github.com/shopspring/decimal` for these types. All three pointers are required; explicit zero values are accepted. `pkg.Rebalance` validates settings before fetching quotes. Treat the pointed-to values as immutable while rebalancing. The transaction lookback remains defined in [tlh.go](internal/rebalance/tlh.go).

## Development

We use `shopspring/decimal` for decimal arithmetic and JSON numbers such as `2.0E-4`. We keep prices and quantities as decimals throughout the calculations and truncate division to 19 decimal places.

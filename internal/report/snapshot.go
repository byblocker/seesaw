package report

import (
	"fmt"
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/shopspring/decimal"
)

func Snapshot(snapshot models.AccountSnapshot) string {
	var output strings.Builder
	account := snapshot.Id[len(snapshot.Id)-4:]

	fmt.Fprintf(&output, "\nAccount %s as of %s\n", account,
		snapshot.CreatedAt.Format(utils.YYYY_MM_DD+" 15:04:05 MST"))
	total := money(snapshot.TotalValue)
	balance := money(snapshot.BuyingPower)
	moneyWidth := max(len(total), len(balance))

	if len(snapshot.Assets) == 0 {
		output.WriteString("\nNo assets.\n\n")
		fmt.Fprintf(&output, "%-8s %*s\n%-8s %*s\n\n", "Balance:", moneyWidth, balance, "Total:", moneyWidth, total)
		return output.String()
	}
	output.WriteByte('\n')
	rows := make([][5]string, 0, len(snapshot.Assets)+1)
	rows = append(rows, [5]string{"Asset", "Value", "Current", "Target", "Deviation"})
	for _, asset := range snapshot.Assets {
		label := "OTHER"
		target := "—"
		deviation := "—"
		if asset.Plan != nil {
			label = strings.Join(slices.Collect(asset.Plan.Tickers()), " / ")
			target = percentage(asset.Plan.TargetAllocation)
			deviation = allocationDeviation(asset.AllocDrift)
		}
		rows = append(rows, [5]string{label, money(asset.Value), percentage(asset.Allocation), target, deviation})
	}
	var widths [5]int
	for _, row := range rows {
		for column, cell := range row {
			widths[column] = max(widths[column], utf8.RuneCountInString(cell))
		}
	}
	tableWidth := 8
	for _, width := range widths {
		tableWidth += width
	}
	for index, row := range rows {
		fmt.Fprintf(&output, "%-*s  %*s  %*s  %*s  %*s\n",
			widths[0], row[0], widths[1], row[1], widths[2], row[2], widths[3], row[3], widths[4], row[4])
		if index == 0 {
			output.WriteString(strings.Repeat("_", tableWidth))
			output.WriteByte('\n')
			continue
		}
		if index < len(rows)-1 {
			output.WriteByte('\n')
		}
	}
	output.WriteString(strings.Repeat("_", tableWidth))
	output.WriteString("\n\n")
	fmt.Fprintf(&output, "%-8s %*s\n%-8s %*s\n", "Balance:", moneyWidth, balance, "Total:", moneyWidth, total)
	output.WriteString(strings.Repeat("_", tableWidth))
	output.WriteString("\n\n")
	return output.String()
}

func money(amount decimal.Decimal) string {
	prefix := "$"
	if amount.IsNegative() {
		prefix = "-$"
	}
	formatted := amount.Abs().StringFixed(2)
	decimalPoint := len(formatted) - 3
	var output strings.Builder
	output.WriteString(prefix)
	for index, digit := range formatted[:decimalPoint] {
		if index > 0 && (decimalPoint-index)%3 == 0 {
			output.WriteByte(',')
		}
		output.WriteRune(digit)
	}
	output.WriteString(formatted[decimalPoint:])
	return output.String()
}

func percentage(ratio decimal.Decimal) string {
	return ratio.Shift(2).StringFixed(2) + "%"
}

func allocationDeviation(drift decimal.Decimal) string {
	if drift.IsPositive() {
		return "+" + percentage(drift)
	}
	return percentage(drift)
}

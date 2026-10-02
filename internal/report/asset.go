package report

import (
	"fmt"
	"seesaw/pkg/models"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/shopspring/decimal"
)

func writeAssets(output *strings.Builder, assets []models.Asset) string {
	headers := [5]string{"Asset", "Value", "Current", "Target", "Deviation"}
	var widths [5]int
	for column, header := range headers {
		widths[column] = len(header)
	}
	rows := make([][5]string, 0, len(assets))
	for _, asset := range assets {
		row := formatAsset(asset)
		rows = append(rows, row)
		for column, cell := range row {
			widths[column] = max(widths[column], utf8.RuneCountInString(cell))
		}
	}
	tableWidth := (len(headers) - 1) * 2
	for _, width := range widths {
		tableWidth += width
	}
	divider := strings.Repeat("_", tableWidth)

	writeAssetRow(output, headers, widths)
	output.WriteString(divider + "\n")
	for index, row := range rows {
		if index > 0 {
			output.WriteByte('\n')
		}
		writeAssetRow(output, row, widths)
	}
	output.WriteString(divider + "\n\n")
	return divider
}

func formatAsset(asset models.Asset) [5]string {
	label, target, deviation := "OTHER", "—", "—"
	if asset.Plan != nil {
		label = strings.Join(slices.Collect(asset.Plan.Tickers()), " / ")
		target = percentage(asset.Plan.TargetAllocation)
		deviation = formatDeviation(asset.AllocDrift)
	}
	return [5]string{label, money(asset.Value), percentage(asset.Allocation), target, deviation}
}

func formatDeviation(drift decimal.Decimal) string {
	if drift.IsPositive() {
		return "+" + percentage(drift)
	}
	return percentage(drift)
}

func writeAssetRow(output *strings.Builder, row [5]string, widths [5]int) {
	fmt.Fprintf(output, "%-*s  %*s  %*s  %*s  %*s\n",
		widths[0], row[0], widths[1], row[1], widths[2], row[2], widths[3], row[3], widths[4], row[4])
}

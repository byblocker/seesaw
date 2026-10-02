package report

import (
	"seesaw/pkg/models"
	"slices"
	"strings"

	"github.com/shopspring/decimal"
)

func writeAssets(output *strings.Builder, assets []models.Asset) string {
	headers := []string{"Asset", "Value", "Current", "Target", "Deviation"}
	rows := make([][]string, 0, len(assets))
	for _, asset := range assets {
		rows = append(rows, formatAsset(asset))
	}
	return writeTable(output, headers, rows, 1)
}

func formatAsset(asset models.Asset) []string {
	label, target, deviation := "OTHER", "—", "—"
	if asset.Plan != nil {
		label = strings.Join(slices.Collect(asset.Plan.Tickers()), " / ")
		target = percentage(asset.Plan.TargetAllocation)
		deviation = formatDeviation(asset.AllocDrift)
	}
	return []string{label, money(asset.Value), percentage(asset.Allocation), target, deviation}
}

func formatDeviation(drift decimal.Decimal) string {
	if drift.IsPositive() {
		return "+" + percentage(drift)
	}
	return percentage(drift)
}

package report

import (
	"fmt"
	"seesaw/internal/utils"
	"seesaw/pkg/models"
	"strings"
)

func Snapshot(snapshot models.AccountSnapshot) string {
	var output strings.Builder
	account := snapshot.Id[len(snapshot.Id)-4:]

	fmt.Fprintf(&output, "\nAccount %s as of %s\n", account,
		snapshot.CreatedAt.Format(utils.YYYY_MM_DD+" 15:04:05 MST"))
	output.WriteByte('\n')
	divider := ""
	if len(snapshot.Assets) == 0 {
		output.WriteString("No assets.\n\n")
	} else {
		divider = writeAssets(&output, snapshot.Assets)
	}

	total := money(snapshot.TotalValue)
	balance := money(snapshot.BuyingPower)
	moneyWidth := max(len(total), len(balance))
	fmt.Fprintf(&output, "%-8s %*s\n%-8s %*s\n", "Balance:", moneyWidth, balance, "Total:", moneyWidth, total)
	if divider != "" {
		output.WriteString(divider)
		output.WriteByte('\n')
	}
	output.WriteByte('\n')
	return output.String()
}

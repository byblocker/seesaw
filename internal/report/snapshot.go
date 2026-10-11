package report

import (
	"fmt"
	"seesaw/internal/utils"
	"seesaw/pkg/portfolio"
	"strings"
)

func Snapshot(snapshot portfolio.Snapshot) string {
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

	writeSummaries(&output, [][2]string{
		{"Balance:", money(snapshot.Portfolio.Balances.CashBalance)},
		{"Total:", money(snapshot.Portfolio.TotalValue)},
	})
	if divider != "" {
		output.WriteString(divider)
		output.WriteByte('\n')
	}
	output.WriteByte('\n')
	return output.String()
}

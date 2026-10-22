package report

import (
	"fmt"
	"seesaw/pkg/portfolio"
	"strings"
	"time"
)

func Snapshot(snapshot portfolio.Snapshot) string {
	var output strings.Builder
	account := snapshot.Id[len(snapshot.Id)-4:]

	fmt.Fprintf(&output, "\nAccount %s as of %s\n", account,
		snapshot.CreatedAt.Format(time.DateTime+" MST"))
	output.WriteByte('\n')
	divider := ""
	if len(snapshot.Assets) == 0 {
		output.WriteString("No assets.\n\n")
	} else {
		divider = writeAssets(&output, snapshot.Assets)
	}

	writeSummaries(&output, [][2]string{
		{"Balance:", money(snapshot.Portfolio.Balances.CashBalance)},
		{"Total:", money(snapshot.TotalValue)},
	})
	if divider != "" {
		output.WriteString(divider)
		output.WriteByte('\n')
	}
	output.WriteByte('\n')
	return output.String()
}

package report

import (
	"fmt"
	"seesaw/internal/utils"
	"seesaw/pkg/portfolio"
	"strings"
	"time"
)

func Snapshot(snapshot portfolio.Snapshot) string {
	var output strings.Builder
	fmt.Fprintf(&output, "\nAccount %s as of %s\n", utils.AccountSuffix(snapshot.AccountId),
		snapshot.CreatedAt.Format(time.DateTime+" MST"))
	output.WriteByte('\n')
	writePortfolio(&output, snapshot)
	return output.String()
}

// Write the asset table, then the balance and total value followed by any other summary groups.
func writePortfolio(output *strings.Builder, snapshot portfolio.Snapshot, summaries ...[][2]string) {
	divider := ""
	if len(snapshot.Assets) == 0 {
		output.WriteString("No assets.\n\n")
	} else {
		divider = writeAssets(output, snapshot.Assets)
	}

	balances := [][2]string{
		{"Balance:", utils.FormatMoney(snapshot.Balances.CashBalance)},
		{"Total:", utils.FormatMoney(snapshot.TotalValue)},
	}
	writeSummaries(output, append([][][2]string{balances}, summaries...)...)
	if divider != "" {
		output.WriteString(divider)
		output.WriteByte('\n')
	}
	output.WriteByte('\n')
}

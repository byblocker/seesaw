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
	divider := ""
	if len(snapshot.Assets) == 0 {
		output.WriteString("No assets.\n\n")
	} else {
		divider = writeAssets(&output, snapshot.Assets)
	}

	writeSummaries(&output, [][2]string{
		{"Balance:", utils.FormatMoney(snapshot.Balances.CashBalance)},
		{"Total:", utils.FormatMoney(snapshot.TotalValue)},
	})
	if divider != "" {
		output.WriteString(divider)
		output.WriteByte('\n')
	}
	output.WriteByte('\n')
	return output.String()
}

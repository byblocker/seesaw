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

// Write the asset table, then any summary groups.
func writePortfolio(output *strings.Builder, snapshot portfolio.Snapshot, summaries ...[][2]string) {
	writeAssets(output, snapshot)
	if len(summaries) > 0 {
		writeSummaries(output, summaries...)
		output.WriteByte('\n')
	}
}

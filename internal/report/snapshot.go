package report

import (
	"fmt"
	"seesaw/internal/format"
	"seesaw/pkg/portfolio"
	"strings"
	"time"
)

func Snapshot(snapshot portfolio.Snapshot) string {
	var output strings.Builder
	fmt.Fprintf(&output, "\nAccount %s as of %s\n", format.AccountSuffix(snapshot.AccountId),
		snapshot.CreatedAt.Format(time.DateTime+" MST"))
	output.WriteByte('\n')
	writePortfolio(&output, snapshot)
	return output.String()
}

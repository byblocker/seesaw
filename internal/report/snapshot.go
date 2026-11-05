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

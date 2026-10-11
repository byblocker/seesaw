package liquidation

import (
	"context"
	"fmt"
	"testing"
)

// Report nodes visited per search alongside time, since node counts track pruning
// regardless of machine speed.
func BenchmarkRealisticSearch(b *testing.B) {
	for _, scenario := range []struct{ amount, maxExcessCash, maxDeviation string }{
		{"1000", "100", "0.02"},
		{"7000", "100", "0.02"},
		{"7000", "500", "0.02"},
		{"7000", "500", "0.10"},
		{"25000", "500", "0.02"},
		{"25000", "2000", "0.10"},
	} {
		name := fmt.Sprintf("amount=%s/excess=%s/deviation=%s", scenario.amount, scenario.maxExcessCash, scenario.maxDeviation)
		b.Run(name, func(b *testing.B) {
			nodes := 0
			for i := range b.N {
				p, request := realisticCase(int64(i%5), scenario.amount, scenario.maxExcessCash, scenario.maxDeviation)
				search, err := p.search(request)
				if err != nil {
					b.Fatal(err)
				}
				if err := search.run(context.Background()); err != nil {
					b.Fatal(err)
				}
				nodes += search.visitedNodes
			}
			b.ReportMetric(float64(nodes)/float64(b.N), "nodes/op")
		})
	}
}

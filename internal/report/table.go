package report

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

func writeTable(output *strings.Builder, headers []string, rows [][]string, textColumns int) string {
	widths, divider := measureTable(headers, rows)

	writeTableRow(output, headers, widths, textColumns)
	output.WriteString(divider)
	output.WriteByte('\n')

	for index, row := range rows {
		if index > 0 {
			output.WriteByte('\n')
		}
		writeTableRow(output, row, widths, textColumns)
	}

	output.WriteString(divider)
	output.WriteString("\n\n")
	return divider
}

func measureTable(headers []string, rows [][]string) ([]int, string) {
	widths := make([]int, len(headers))
	for column, header := range headers {
		widths[column] = utf8.RuneCountInString(header)
	}

	for _, row := range rows {
		for column, cell := range row {
			widths[column] = max(widths[column], utf8.RuneCountInString(cell))
		}
	}

	tableWidth := (len(headers) - 1) * 2
	for _, width := range widths {
		tableWidth += width
	}

	return widths, strings.Repeat("_", tableWidth)
}

func writeTableRow(output *strings.Builder, row []string, widths []int, textColumns int) {
	for column, cell := range row {
		if column > 0 {
			output.WriteString("  ")
		}

		width := widths[column]
		if column < textColumns {
			width = -width
		}
		fmt.Fprintf(output, "%*s", width, cell)
	}
	output.WriteByte('\n')
}

// Align every group's labels and amounts together, separating groups with a blank line.
func writeSummaries(output *strings.Builder, groups ...[][2]string) {
	var labelWidth, amountWidth int
	for _, summaries := range groups {
		for _, summary := range summaries {
			labelWidth = max(labelWidth, len(summary[0]))
			amountWidth = max(amountWidth, len(summary[1]))
		}
	}

	for i, summaries := range groups {
		if i > 0 {
			output.WriteByte('\n')
		}
		for _, summary := range summaries {
			fmt.Fprintf(output, "%-*s %*s\n", labelWidth, summary[0], amountWidth, summary[1])
		}
	}
}

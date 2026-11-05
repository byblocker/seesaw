package report

import (
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// A borderless table with rules under its header and above any footer, its first
// textColumns columns aligned left and the rest aligned right.
func newTable(headers table.Row, textColumns int) table.Writer {
	style := table.StyleLight
	style.Box.PaddingLeft = ""
	style.Box.PaddingRight = ""
	style.Box.MiddleVertical = "  "
	style.Box.MiddleSeparator = "──"
	style.Format.Header = text.FormatDefault
	style.Format.Footer = text.FormatDefault
	style.Options = table.Options{SeparateColumns: true, SeparateHeader: true, SeparateFooter: true}

	writer := table.NewWriter()
	writer.SetStyle(style)
	writer.SuppressTrailingSpaces()
	writer.AppendHeader(headers)

	columns := make([]table.ColumnConfig, 0, len(headers)-textColumns)
	for number := textColumns + 1; number <= len(headers); number++ {
		columns = append(columns, table.ColumnConfig{
			Number:      number,
			Align:       text.AlignRight,
			AlignHeader: text.AlignRight,
			AlignFooter: text.AlignRight,
		})
	}
	writer.SetColumnConfigs(columns)
	return writer
}

func writeTable(output *strings.Builder, writer table.Writer) {
	output.WriteString(writer.Render())
	output.WriteString("\n\n")
}

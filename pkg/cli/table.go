package cli

import (
	"fmt"
	"io"
	"strings"
)

// writeTable prints headers and rows as aligned columns: two spaces
// between columns, each column as wide as its longest cell. The
// column set matches the TUI view, for humans debugging agent calls.
func writeTable(headers []string, rows [][]string, w io.Writer) error {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	writeRow := func(cells []string) error {
		var b strings.Builder
		for i, cell := range cells {
			if i > 0 {
				b.WriteString("  ")
			}
			b.WriteString(cell)
			if i < len(widths) {
				b.WriteString(strings.Repeat(" ", widths[i]-len(cell)))
			}
		}
		_, err := fmt.Fprintln(w, strings.TrimRight(b.String(), " "))
		return err
	}
	if err := writeRow(headers); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writeRow(row); err != nil {
			return err
		}
	}
	return nil
}

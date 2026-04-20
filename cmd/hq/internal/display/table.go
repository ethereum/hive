// Package display provides terminal output helpers for hq: a minimal aligned
// table renderer and colorized diff formatters.
package display

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Table renders aligned columns to stdout.
type Table struct {
	headers []string
	rows    [][]string
	w       io.Writer
}

// NewTable creates a new table with the given headers.
func NewTable(headers []string) *Table {
	return &Table{headers: headers, w: os.Stdout}
}

// Append adds a row to the table.
func (t *Table) Append(row []string) {
	t.rows = append(t.rows, row)
}

// Render prints the table with aligned columns.
func (t *Table) Render() {
	if len(t.headers) == 0 {
		return
	}

	// Calculate column widths
	widths := make([]int, len(t.headers))
	for i, h := range t.headers {
		widths[i] = len(h)
	}
	for _, row := range t.rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header
	printRow(t.w, t.headers, widths)
	// Print separator
	parts := make([]string, len(widths))
	for i, w := range widths {
		parts[i] = strings.Repeat("-", w)
	}
	fmt.Fprintln(t.w, strings.Join(parts, "  "))
	// Print rows
	for _, row := range t.rows {
		printRow(t.w, row, widths)
	}
}

func printRow(w io.Writer, cells []string, widths []int) {
	parts := make([]string, len(widths))
	for i := range widths {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		if i < len(widths)-1 {
			parts[i] = fmt.Sprintf("%-*s", widths[i], cell)
		} else {
			parts[i] = cell // No padding on last column
		}
	}
	fmt.Fprintln(w, strings.Join(parts, "  "))
}

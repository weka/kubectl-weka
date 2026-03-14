package cmd

import (
	"github.com/jedib0t/go-pretty/v6/table"
)

// TableData holds headers and rows for rendering a table
// Each row is a map of column name to value
// Headers order determines column order
// Rows is a slice of maps, each map is a row
// Values can be string or any fmt.Stringer

type TableData struct {
	Headers []string
	Rows    []map[string]interface{}
}

// RenderTable renders TableData as a pretty table with given indentation and style
func RenderTable(data TableData, indent int, style table.Style) string {
	t := table.NewWriter()
	t.SetStyle(style)
	if style.Name == table.StyleLight.Name {
		styleTableMinimal(t)
	}

	// Build header row
	headerRow := table.Row{}
	for _, col := range data.Headers {
		headerRow = append(headerRow, col)
	}
	t.AppendHeader(headerRow)

	// Build rows
	for _, rowMap := range data.Rows {
		row := table.Row{}
		for _, col := range data.Headers {
			val := rowMap[col]
			row = append(row, val)
		}
		t.AppendRow(row)
	}

	output := t.Render()
	if indent > 0 {
		output = indentText(output, indent)
	}
	return output
}

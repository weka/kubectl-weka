package printer

import (
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
)

// HideEmptyColumns hides all columns in the provided headers/rows that are empty in all rows.
func HideEmptyColumns(t table.Writer, headers []string, rows [][]interface{}) {
	if len(headers) == 0 || len(rows) == 0 {
		return
	}

	colEmpty := make([]bool, len(headers))
	for i := range colEmpty {
		colEmpty[i] = true
	}

	for _, row := range rows {
		for i, cell := range row {
			if cell != nil && cell != "" {
				colEmpty[i] = false
			}
		}
	}

	var configs []table.ColumnConfig
	for i := range headers {
		if colEmpty[i] {
			configs = append(configs, table.ColumnConfig{
				Number: i,
				Hidden: true,
			})
		}
	}
	t.SetColumnConfigs(configs)
}

// SetVisibleColumns sets only the specified columns as visible, hiding others.
func SetVisibleColumns(t table.Writer, headers []string, columnNames ...string) {
	visible := map[string]bool{}
	for _, col := range columnNames {
		visible[col] = true
	}
	var configs []table.ColumnConfig
	for i, col := range headers {
		configs = append(configs, table.ColumnConfig{
			Number: i,
			Hidden: !visible[col],
		})
	}
	t.SetColumnConfigs(configs)
}

// HideColumn hides a specific column by name.
func HideColumn(t table.Writer, headers []string, columnName string) {
	for i, col := range headers {
		if col == columnName {
			t.SetColumnConfigs([]table.ColumnConfig{{Number: i, Hidden: true}})
			break
		}
	}
}

// ShowColumn shows a specific column by name.
func ShowColumn(t table.Writer, headers []string, columnName string) {
	for i, col := range headers {
		if col == columnName {
			t.SetColumnConfigs([]table.ColumnConfig{{Number: i, Hidden: false}})
			break
		}
	}
}

// indentText indents a block of text by the specified number of spaces
func indentText(text string, spaces int, subsequentSpace ...int) string {
	if spaces <= 0 || text == "" {
		return text
	}

	indent := strings.Repeat(" ", spaces)
	subIndent := indent
	lines := strings.Split(text, "\n")

	if subsequentSpace != nil {
		subIndent = strings.Repeat(" ", subsequentSpace[0]) + subIndent
	}
	var result []string
	for i, line := range lines {
		if line == "" {
			result = append(result, "")
		} else if i > 0 {
			result = append(result, subIndent+line)
		} else {
			result = append(result, indent+line)
		}
	}

	return strings.Join(result, "\n")
}

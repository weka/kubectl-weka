package printer

import (
	"fmt"
	"github.com/weka/kubectl-weka/pkg/utils"
	"io"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
)

type TablePrinter struct {
	opts PrinterOptions
}

func (tp *TablePrinter) SetOptions(opts PrinterOptions) {
	tp.opts = opts
}

func (tp *TablePrinter) GetOptions() PrinterOptions {
	return tp.opts
}

func (tp *TablePrinter) Print(columns []TableColumn, rows []TableRow, w io.Writer) error {
	opts := tp.opts
	hideCols := map[string]struct{}{}
	for _, name := range opts.HideColumnsList {
		hideCols[strings.ToUpper(name)] = struct{}{}
	}
	visibleCols := []TableColumn{}
	colNames := map[string]struct{}{}
	if len(opts.ColumnsList) > 0 {
		for _, name := range opts.ColumnsList {
			colNames[strings.ToUpper(name)] = struct{}{}
		}
	}
	for _, col := range columns {
		colNameUpper := strings.ToUpper(col.Name)
		if _, hidden := hideCols[colNameUpper]; hidden {
			continue
		}
		if len(opts.ColumnsList) > 0 {
			if _, ok := colNames[colNameUpper]; !ok {
				continue
			}
		}
		if !opts.WideOutput && col.VisibleInWide {
			continue
		}
		visibleCols = append(visibleCols, col)
	}
	tw := table.NewWriter()
	switch tp.opts.TableStyle {
	case TableStyleMinimal:
		tw.SetStyle(table.StyleLight)
		tw.Style().Options.DrawBorder = false
		tw.Style().Options.SeparateRows = false
		tw.Style().Options.SeparateColumns = false
		tw.Style().Options.SeparateHeader = false
		tw.Style().Box.PaddingLeft = ""
		tw.Style().Box.PaddingRight = "  "
	case TableStyleRoundedBox:
		tw.SetStyle(table.StyleRounded)
	}

	if opts.HideEmptyColumns && len(rows) > 0 {
		filteredCols := make([]TableColumn, 0, len(visibleCols))
		for _, col := range visibleCols {
			allEmpty := true
			for _, row := range rows {
				val := row.Values[col.Name]
				strVal := fmt.Sprintf("%v", val)
				if col.FormatFuncs != nil && len(col.FormatFuncs) > 0 {
					for _, fn := range col.FormatFuncs {
						strVal = fn(val)
					}
				}
				if strVal != "" && strVal != "<none>" {
					allEmpty = false
					break
				}
			}
			if !allEmpty {
				filteredCols = append(filteredCols, col)
			}
		}
		visibleCols = filteredCols
	}
	if opts.ShowHeader {
		headers := table.Row{}
		for _, col := range visibleCols {
			headers = append(headers, col.Name)
		}
		tw.AppendHeader(headers)
	}
	if len(rows) == 0 {
		_, _ = fmt.Fprintln(w, "No resources found.")
		return nil
	}
	for _, row := range rows {
		vals := table.Row{}
		for _, col := range visibleCols {
			val := row.Values[col.Name]
			strVal := fmt.Sprintf("%v", val)
			if col.FormatFuncs != nil && len(col.FormatFuncs) > 0 {
				for _, fn := range col.FormatFuncs {
					strVal = fn(val)
				}
			}
			if strVal == "" {
				strVal = "<none>"
			}
			vals = append(vals, strVal)
		}
		tw.AppendRow(vals)
	}
	var sb strings.Builder
	tw.SetOutputMirror(&sb)
	tw.Render()
	tableStr := sb.String()
	if opts.IndentByNumSpaces > 0 {
		tableStr = utils.IndentText(tableStr, opts.IndentByNumSpaces)
	}
	_, err := io.WriteString(w, tableStr)
	return err
}

func NewSupportBundlePrinter() ResourcePrinter {
	printer := &TablePrinter{opts: PrinterOptions{
		ShowHeader: true,
		WideOutput: true,
		TableStyle: TableStyleMinimal,
	}}
	return printer
}

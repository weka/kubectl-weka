package cmd

import (
	"io"
)

type TableColumn struct {
	Name                 string
	VisibleInWide        bool
	TableFormatFunctions []func(interface{}) string // functions to refine for table view
}

type TableRow struct {
	Values map[string]interface{}
}

type TableStyle string

const (
	TableStyleMinimal    TableStyle = "minimal"
	TableStyleRoundedBox TableStyle = "roundedBox"
)

type PrinterOptions struct {
	ShowHeader        bool
	WideOutput        bool
	ColumnsList       []string
	HideColumnsList   []string // columns to hide, case-insensitive
	HideEmptyColumns  bool     // omit columns that are empty in all rows
	IndentByNumSpaces int      // if >0, indent table output by this many spaces
	TableStyle        TableStyle
}

type ResourcePrinter interface {
	SetOptions(opts PrinterOptions)
	Print(columns []TableColumn, rows []TableRow, w io.Writer) error
}

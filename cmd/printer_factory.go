package cmd

import (
	"strings"
)

// GetPrinterFromFlags parses output flag and other options, returns configured ResourcePrinter and columnsList
func GetPrinterFromFlags(outputFlag string, showHeader bool, hideColumnsList []string, hideEmptyColumns bool, indentByNumSpaces int, tableStyle TableStyle) (ResourcePrinter, []string) {
	output := strings.ToLower(strings.TrimSpace(outputFlag))
	wideOutput := false
	columnsList := []string{}
	if strings.HasPrefix(output, "custom-columns=") {
		customCols := strings.TrimPrefix(output, "custom-columns=")
		for _, col := range strings.Split(customCols, ",") {
			col = strings.ToUpper(strings.TrimSpace(col))
			if col != "" {
				columnsList = append(columnsList, col)
			}
		}
		output = "table"
	}
	if output == "wide" {
		wideOutput = true
		output = "table"
	}
	opts := PrinterOptions{
		ShowHeader:        showHeader,
		WideOutput:        wideOutput,
		ColumnsList:       columnsList,
		HideColumnsList:   hideColumnsList,
		HideEmptyColumns:  hideEmptyColumns,
		IndentByNumSpaces: indentByNumSpaces,
		TableStyle:        tableStyle,
	}

	var printer ResourcePrinter
	switch output {
	case "json", "json-pretty":
		printer = &JsonPrinter{}
	case "yaml":
		printer = &YamlPrinter{}
	default:
		printer = &TablePrinter{}
	}
	printer.SetOptions(opts)
	return printer, columnsList
}

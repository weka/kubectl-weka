package printer

import (
	"bytes"
	"testing"
)

func TestGetPrinterFromFlags(t *testing.T) {
	tests := []struct {
		name            string
		outputFlag      string
		expectedType    string
		expectedColumns int
	}{
		{
			name:            "default table",
			outputFlag:      "table",
			expectedType:    "*printer.TablePrinter",
			expectedColumns: 0,
		},
		{
			name:            "wide output",
			outputFlag:      "wide",
			expectedType:    "*printer.TablePrinter",
			expectedColumns: 0,
		},
		{
			name:            "json output",
			outputFlag:      "json",
			expectedType:    "*printer.JsonPrinter",
			expectedColumns: 0,
		},
		{
			name:            "yaml output",
			outputFlag:      "yaml",
			expectedType:    "*printer.YamlPrinter",
			expectedColumns: 0,
		},
		{
			name:            "custom columns",
			outputFlag:      "custom-columns=name,age,status",
			expectedType:    "*printer.TablePrinter",
			expectedColumns: 3,
		},
		{
			name:            "case insensitive",
			outputFlag:      "JSON",
			expectedType:    "*printer.JsonPrinter",
			expectedColumns: 0,
		},
		{
			name:            "with whitespace",
			outputFlag:      "  yaml  ",
			expectedType:    "*printer.YamlPrinter",
			expectedColumns: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			printer, cols := GetPrinterFromFlags(tt.outputFlag, true, nil, false, 0, TableStyleMinimal)

			// Check printer type
			printerType := getTypeName(printer)
			if printerType != tt.expectedType {
				t.Errorf("GetPrinterFromFlags(%q) returned %s, want %s", tt.outputFlag, printerType, tt.expectedType)
			}

			// Check columns
			if len(cols) != tt.expectedColumns {
				t.Errorf("GetPrinterFromFlags(%q) returned %d columns, want %d", tt.outputFlag, len(cols), tt.expectedColumns)
			}
		})
	}
}

func TestTablePrinterOptions(t *testing.T) {
	t.Run("set and get options", func(t *testing.T) {
		printer := &TablePrinter{}
		opts := PrinterOptions{
			ShowHeader:       true,
			WideOutput:       true,
			HideEmptyColumns: true,
		}

		printer.SetOptions(opts)
		retrieved := printer.GetOptions()

		if retrieved.ShowHeader != opts.ShowHeader ||
			retrieved.WideOutput != opts.WideOutput ||
			retrieved.HideEmptyColumns != opts.HideEmptyColumns {
			t.Error("Options not properly set/retrieved")
		}
	})
}

func TestTableColumnAndRow(t *testing.T) {
	t.Run("table column", func(t *testing.T) {
		col := TableColumn{
			Name:          "Name",
			VisibleInWide: true,
		}
		if col.Name != "Name" || !col.VisibleInWide {
			t.Error("TableColumn not properly initialized")
		}
	})

	t.Run("table row values", func(t *testing.T) {
		row := TableRow{
			Values: map[string]interface{}{
				"name": "John",
				"age":  30,
			},
		}
		if row.Values["name"] != "John" || row.Values["age"] != 30 {
			t.Error("TableRow values not properly set")
		}
	})
}

func TestPrinterWithEmptyData(t *testing.T) {
	printer := &TablePrinter{}
	printer.SetOptions(PrinterOptions{ShowHeader: true})

	var buf bytes.Buffer

	t.Run("empty columns", func(t *testing.T) {
		err := printer.Print([]TableColumn{}, []TableRow{}, &buf)
		if err != nil {
			t.Errorf("Print with empty columns failed: %v", err)
		}
	})

	t.Run("empty rows", func(t *testing.T) {
		columns := []TableColumn{
			{Name: "Name"},
			{Name: "Age"},
		}
		err := printer.Print(columns, []TableRow{}, &buf)
		if err != nil {
			t.Errorf("Print with empty rows failed: %v", err)
		}
	})
}

func TestTableStyleConstants(t *testing.T) {
	tests := []struct {
		name     string
		style    TableStyle
		expected string
	}{
		{
			name:     "minimal style",
			style:    TableStyleMinimal,
			expected: "minimal",
		},
		{
			name:     "rounded box style",
			style:    TableStyleRoundedBox,
			expected: "roundedBox",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.style) != tt.expected {
				t.Errorf("TableStyle %v = %q, want %q", tt.style, string(tt.style), tt.expected)
			}
		})
	}
}

// Helper function to get type name as string
func getTypeName(v interface{}) string {
	// Simple type assertion check
	switch v.(type) {
	case *TablePrinter:
		return "*printer.TablePrinter"
	case *JsonPrinter:
		return "*printer.JsonPrinter"
	case *YamlPrinter:
		return "*printer.YamlPrinter"
	default:
		return "unknown"
	}
}

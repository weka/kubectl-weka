package cmd

import (
	"encoding/json"
	"io"
)

type JsonPrinter struct {
	opts PrinterOptions
}

func (jp *JsonPrinter) SetOptions(opts PrinterOptions) {
	jp.opts = opts
}

func (jp *JsonPrinter) Print(_ []TableColumn, rows []TableRow, w io.Writer) error {
	if len(rows) == 0 {
		_, err := w.Write([]byte("[]\n"))
		return err
	}
	// For JSON, marshal all rows as a list of maps
	out := make([]map[string]interface{}, len(rows))
	for i, row := range rows {
		out[i] = row.Values
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

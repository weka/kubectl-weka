package cmd

import (
	"gopkg.in/yaml.v3"
	"io"
)

type YamlPrinter struct {
	opts PrinterOptions
}

func (yp *YamlPrinter) SetOptions(opts PrinterOptions) {
	yp.opts = opts
}

func (yp *YamlPrinter) Print(_ []TableColumn, rows []TableRow, w io.Writer) error {
	if len(rows) == 0 {
		_, err := w.Write([]byte("[]\n"))
		return err
	}
	out := make([]map[string]interface{}, len(rows))
	for i, row := range rows {
		out[i] = row.Values
	}
	enc := yaml.NewEncoder(w)
	defer func() { _ = enc.Close() }()
	return enc.Encode(out)
}

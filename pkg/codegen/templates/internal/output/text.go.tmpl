package output

import (
	"encoding/json"
	"fmt"
	"strings"
)

// TextFormatter outputs data as plain text.
type TextFormatter struct{}

func (f *TextFormatter) FormatItem(v any) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *TextFormatter) FormatList(headers []string, rows [][]string) (string, error) {
	if len(rows) == 0 {
		return "No results.", nil
	}

	var b strings.Builder
	for i, row := range rows {
		if i > 0 {
			b.WriteString("\n")
		}
		for j, h := range headers {
			if j < len(row) {
				fmt.Fprintf(&b, "%s: %s\n", h, row[j])
			}
		}
	}
	return b.String(), nil
}

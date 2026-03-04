package output

import (
	"encoding/json"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	cellStyle   = lipgloss.NewStyle()
)

// TableFormatter outputs data as a styled lipgloss table.
type TableFormatter struct{}

func (f *TableFormatter) FormatItem(v any) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *TableFormatter) FormatList(headers []string, rows [][]string) (string, error) {
	t := table.New().
		Headers(headers...).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("8"))).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		})

	for _, row := range rows {
		t.Row(row...)
	}

	return t.Render(), nil
}

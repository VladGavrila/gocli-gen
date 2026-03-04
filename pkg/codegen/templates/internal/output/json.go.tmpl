package output

import "encoding/json"

// JSONFormatter outputs data as JSON.
type JSONFormatter struct{}

func (f *JSONFormatter) FormatItem(v any) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *JSONFormatter) FormatList(headers []string, rows [][]string) (string, error) {
	result := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		m := make(map[string]string)
		for i, h := range headers {
			if i < len(row) {
				m[h] = row[i]
			}
		}
		result = append(result, m)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

package naming

// OpenAPITypeToGo maps an OpenAPI type+format to a Go type string.
func OpenAPITypeToGo(typ, format string) string {
	switch typ {
	case "string":
		switch format {
		case "date-time", "date", "time":
			return "string" // Keep as string for simplicity
		case "binary", "byte":
			return "[]byte"
		default:
			return "string"
		}
	case "integer":
		switch format {
		case "int32":
			return "int32"
		case "int64":
			return "int64"
		default:
			return "int"
		}
	case "number":
		switch format {
		case "float":
			return "float32"
		case "double":
			return "float64"
		default:
			return "float64"
		}
	case "boolean":
		return "bool"
	case "object":
		return "map[string]any"
	case "array":
		return "[]any"
	default:
		return "any"
	}
}

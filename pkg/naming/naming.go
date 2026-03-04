package naming

import (
	"strings"
	"unicode"
)

// Common acronyms that should be fully uppercase in Go names.
var acronyms = map[string]bool{
	"id": true, "url": true, "uri": true, "api": true,
	"http": true, "https": true, "html": true, "json": true,
	"xml": true, "sql": true, "ssh": true, "tcp": true,
	"udp": true, "ip": true, "dns": true, "tls": true,
	"ssl": true, "cpu": true, "ram": true, "os": true,
	"ui": true, "uuid": true, "uid": true, "gid": true,
	"pid": true, "mac": true, "vm": true, "io": true,
	"eof": true, "qps": true, "ttl": true, "acl": true,
}

// Irregular plurals.
var irregularPlurals = map[string]string{
	"person":   "people",
	"child":    "children",
	"man":      "men",
	"woman":    "women",
	"mouse":    "mice",
	"goose":    "geese",
	"foot":     "feet",
	"tooth":    "teeth",
	"datum":    "data",
	"status":   "statuses",
	"bus":      "buses",
	"index":    "indices",
	"matrix":   "matrices",
	"vertex":   "vertices",
	"analysis": "analyses",
}

// ToGoName converts a string to a Go exported name.
// Examples: "project_type" → "ProjectType", "id" → "ID", "user" → "User"
func ToGoName(s string) string {
	if s == "" {
		return ""
	}
	words := splitWords(s)
	var result strings.Builder
	for _, w := range words {
		lower := strings.ToLower(w)
		if acronyms[lower] {
			result.WriteString(strings.ToUpper(w))
		} else {
			result.WriteString(strings.ToUpper(w[:1]) + strings.ToLower(w[1:]))
		}
	}
	return result.String()
}

// ToGoFieldName converts a JSON field name to a Go struct field name.
// Same as ToGoName but handles special cases for common field names.
func ToGoFieldName(s string) string {
	return ToGoName(s)
}

// ToCLIName converts a string to a CLI-friendly name (lowercase, hyphenated).
// Examples: "ProjectType" → "project-type", "userId" → "user-id"
func ToCLIName(s string) string {
	words := splitWords(s)
	for i, w := range words {
		words[i] = strings.ToLower(w)
	}
	return strings.Join(words, "-")
}

// ToSnakeCase converts a string to snake_case.
// Examples: "ProjectType" → "project_type", "userId" → "user_id"
func ToSnakeCase(s string) string {
	words := splitWords(s)
	for i, w := range words {
		words[i] = strings.ToLower(w)
	}
	return strings.Join(words, "_")
}

// ToPlural returns the plural form of a word.
func ToPlural(s string) string {
	if s == "" {
		return ""
	}
	lower := strings.ToLower(s)
	if p, ok := irregularPlurals[lower]; ok {
		// Preserve original casing
		if s[0] >= 'A' && s[0] <= 'Z' {
			return strings.ToUpper(p[:1]) + p[1:]
		}
		return p
	}
	// Standard English rules
	switch {
	case strings.HasSuffix(lower, "s") || strings.HasSuffix(lower, "x") ||
		strings.HasSuffix(lower, "z") || strings.HasSuffix(lower, "ch") ||
		strings.HasSuffix(lower, "sh"):
		return s + "es"
	case strings.HasSuffix(lower, "y") && len(lower) > 1 && !isVowel(rune(lower[len(lower)-2])):
		return s[:len(s)-1] + "ies"
	case strings.HasSuffix(lower, "f"):
		return s[:len(s)-1] + "ves"
	case strings.HasSuffix(lower, "fe"):
		return s[:len(s)-2] + "ves"
	default:
		return s + "s"
	}
}

// splitWords splits a string into words by camelCase, PascalCase, snake_case, or kebab-case boundaries.
func splitWords(s string) []string {
	var words []string
	var current strings.Builder

	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		switch {
		case r == '_' || r == '-' || r == '.' || r == ' ':
			// Separator
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		case unicode.IsUpper(r):
			// Check for acronym sequences (e.g., "ID", "URL", "HTTP")
			if current.Len() > 0 {
				// If previous was lowercase, start new word
				if i > 0 && unicode.IsLower(runes[i-1]) {
					words = append(words, current.String())
					current.Reset()
				} else if i+1 < len(runes) && unicode.IsLower(runes[i+1]) && current.Len() > 1 {
					// End of acronym (e.g., "HTTPServer" → "HTTP", "Server")
					word := current.String()
					words = append(words, word[:len(word)-1])
					current.Reset()
					current.WriteRune(rune(word[len(word)-1]))
				}
			}
			current.WriteRune(r)
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

func isVowel(r rune) bool {
	switch r {
	case 'a', 'e', 'i', 'o', 'u', 'A', 'E', 'I', 'O', 'U':
		return true
	}
	return false
}

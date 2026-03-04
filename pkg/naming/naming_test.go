package naming

import "testing"

func TestToGoName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"project", "Project"},
		{"project_type", "ProjectType"},
		{"id", "ID"},
		{"user_id", "UserID"},
		{"userId", "UserID"},
		{"url", "URL"},
		{"apiKey", "APIKey"},
		{"httpServer", "HTTPServer"},
		{"some-value", "SomeValue"},
		{"jsonResponse", "JSONResponse"},
		{"", ""},
		{"a", "A"},
		{"IP", "IP"},
		{"htmlCleanupBlob", "HTMLCleanupBlob"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToGoName(tt.input)
			if got != tt.want {
				t.Errorf("ToGoName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestToCLIName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ProjectType", "project-type"},
		{"userId", "user-id"},
		{"URL", "url"},
		{"project", "project"},
		{"some_thing", "some-thing"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToCLIName(tt.input)
			if got != tt.want {
				t.Errorf("ToCLIName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ProjectType", "project_type"},
		{"userId", "user_id"},
		{"URL", "url"},
		{"some-thing", "some_thing"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToSnakeCase(tt.input)
			if got != tt.want {
				t.Errorf("ToSnakeCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestToPlural(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"project", "projects"},
		{"user", "users"},
		{"status", "statuses"},
		{"category", "categories"},
		{"bus", "buses"},
		{"child", "children"},
		{"person", "people"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToPlural(tt.input)
			if got != tt.want {
				t.Errorf("ToPlural(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

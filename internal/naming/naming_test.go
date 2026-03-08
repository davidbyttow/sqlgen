package naming

import "testing"

func TestToPascal(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"user_id", "UserID"},
		{"created_at", "CreatedAt"},
		{"http_url", "HTTPURL"},
		{"organization", "Organization"},
		{"post_tags", "PostTags"},
		{"id", "ID"},
		{"uuid", "UUID"},
		{"html_body", "HTMLBody"},
		{"api_key", "APIKey"},
		{"is_published", "IsPublished"},
		{"", ""},
		{"a", "A"},
		{"json_data", "JSONData"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToPascal(tt.input)
			if got != tt.want {
				t.Errorf("ToPascal(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestToCamel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"user_id", "userID"},
		{"created_at", "createdAt"},
		{"organization", "organization"},
		{"id", "id"},
		{"api_key", "apiKey"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToCamel(tt.input)
			if got != tt.want {
				t.Errorf("ToCamel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestToSnake(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"UserID", "user_id"},
		{"CreatedAt", "created_at"},
		{"HTTPURL", "http_url"},
		{"Organization", "organization"},
		{"HTMLBody", "html_body"},
		{"postTags", "post_tags"},
		{"already_snake", "already_snake"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToSnake(tt.input)
			if got != tt.want {
				t.Errorf("ToSnake(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"user", "users"},
		{"post", "posts"},
		{"tag", "tags"},
		{"category", "categories"},
		{"status", "statuses"},
		{"church", "churches"},
		{"box", "boxes"},
		{"bus", "buses"},
		{"person", "people"},
		{"child", "children"},
		{"leaf", "leaves"},
		{"knife", "knives"},
		{"", ""},
		{"index", "indexes"},
		{"alias", "aliases"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Pluralize(tt.input)
			if got != tt.want {
				t.Errorf("Pluralize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSingularize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"users", "user"},
		{"posts", "post"},
		{"tags", "tag"},
		{"categories", "category"},
		{"statuses", "status"},
		{"churches", "church"},
		{"boxes", "box"},
		{"buses", "bus"},
		{"people", "person"},
		{"children", "child"},
		{"leaves", "leaf"},
		{"", ""},
		{"indexes", "index"},
		{"aliases", "alias"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Singularize(tt.input)
			if got != tt.want {
				t.Errorf("Singularize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSafeGoName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"type", "type_"},
		{"select", "select_"},
		{"name", "name"},
		{"user", "user"},
		{"map", "map_"},
		{"error", "error_"},
		{"string", "string_"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SafeGoName(tt.input)
			if got != tt.want {
				t.Errorf("SafeGoName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitWords(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"user_id", []string{"user", "id"}},
		{"UserID", []string{"User", "ID"}},
		{"HTMLParser", []string{"HTML", "Parser"}},
		{"getHTTPURL", []string{"get", "HTTP", "URL"}},
		{"simple", []string{"simple"}},
		{"a_b_c", []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitWords(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("splitWords(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitWords(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

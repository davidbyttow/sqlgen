package postgres

import "testing"

func TestParseInvalidSQL(t *testing.T) {
	p := &Parser{}

	tests := []struct {
		name string
		sql  string
	}{
		{"syntax error", "CREATE TABL users (id INT);"},
		{"unclosed paren", "CREATE TABLE users (id INT"},
		{"missing semicolon between statements", "CREATE TABLE a (id INT) CREATE TABLE b (id INT)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseString(tt.sql)
			if err == nil {
				t.Error("expected parse error for invalid SQL")
			}
		})
	}
}

func TestParseEmptyInput(t *testing.T) {
	p := &Parser{}

	s, err := p.ParseString("")
	if err != nil {
		t.Fatalf("unexpected error for empty input: %v", err)
	}
	if len(s.Tables) != 0 || len(s.Enums) != 0 || len(s.Views) != 0 {
		t.Error("empty input should produce empty schema")
	}
}

func TestParseCommentsOnly(t *testing.T) {
	p := &Parser{}

	s, err := p.ParseString("-- just a comment\n/* block comment */")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.Tables) != 0 {
		t.Error("comments-only input should produce empty schema")
	}
}

func TestParseNonexistentFile(t *testing.T) {
	p := &Parser{}
	_, err := p.ParseFile("/nonexistent/file.sql")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

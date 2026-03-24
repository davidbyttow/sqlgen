package gen

import (
	"strings"
	"testing"
)

func TestImportsEmpty(t *testing.T) {
	s := NewImportSet()
	got := s.FormatBlock()
	if got != "" {
		t.Errorf("empty import set should produce empty string, got %q", got)
	}
}

func TestImportsSingleStdlib(t *testing.T) {
	s := NewImportSet()
	s.Add("fmt")
	got := s.FormatBlock()
	want := "import (\n\t\"fmt\"\n)"
	if got != want {
		t.Errorf("single stdlib import:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestImportsDeduplication(t *testing.T) {
	s := NewImportSet()
	s.Add("fmt")
	s.Add("fmt")
	s.Add("fmt")
	sorted := s.Sorted()
	if len(sorted) != 1 {
		t.Errorf("expected 1 unique import, got %d: %v", len(sorted), sorted)
	}
}

func TestImportsIgnoreEmpty(t *testing.T) {
	s := NewImportSet()
	s.Add("")
	s.Add("fmt")
	s.Add("")
	sorted := s.Sorted()
	if len(sorted) != 1 {
		t.Errorf("expected 1 import (empty strings ignored), got %d: %v", len(sorted), sorted)
	}
}

func TestImportsBasicCollection(t *testing.T) {
	s := NewImportSet()
	s.Add("fmt")
	s.Add("strings")
	s.Add("time")
	sorted := s.Sorted()
	if len(sorted) != 3 {
		t.Fatalf("expected 3 imports, got %d", len(sorted))
	}
	if sorted[0] != "fmt" || sorted[1] != "strings" || sorted[2] != "time" {
		t.Errorf("expected [fmt strings time], got %v", sorted)
	}
}

func TestImportsStdlibAndThirdPartyGrouping(t *testing.T) {
	s := NewImportSet()
	s.Add("time")
	s.Add("fmt")
	s.Add("github.com/google/uuid")
	s.Add("database/sql")
	s.Add("github.com/lib/pq")

	got := s.FormatBlock()

	lines := strings.Split(got, "\n")

	sepIdx := -1
	for i, l := range lines {
		if l == "" {
			sepIdx = i
			break
		}
	}
	if sepIdx == -1 {
		t.Fatal("expected blank line separating stdlib from third-party imports")
	}

	stdlibSection := strings.Join(lines[1:sepIdx], "\n")
	if !strings.Contains(stdlibSection, `"database/sql"`) {
		t.Errorf("stdlib section missing database/sql")
	}
	if !strings.Contains(stdlibSection, `"fmt"`) {
		t.Errorf("stdlib section missing fmt")
	}
	if !strings.Contains(stdlibSection, `"time"`) {
		t.Errorf("stdlib section missing time")
	}

	thirdPartySection := strings.Join(lines[sepIdx+1:], "\n")
	if !strings.Contains(thirdPartySection, `"github.com/google/uuid"`) {
		t.Errorf("third-party section missing github.com/google/uuid")
	}
	if !strings.Contains(thirdPartySection, `"github.com/lib/pq"`) {
		t.Errorf("third-party section missing github.com/lib/pq")
	}

	if strings.Contains(thirdPartySection, `"fmt"`) {
		t.Error("fmt leaked into third-party section")
	}
}

func TestImportsMixedSortOrder(t *testing.T) {
	s := NewImportSet()
	s.Add("strings")
	s.Add("fmt")
	s.Add("github.com/z/z")
	s.Add("github.com/a/a")

	got := s.FormatBlock()
	lines := strings.Split(got, "\n")

	var content []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" && trimmed != "import (" && trimmed != ")" {
			content = append(content, trimmed)
		}
	}

	expected := []string{`"fmt"`, `"strings"`, `"github.com/a/a"`, `"github.com/z/z"`}
	if len(content) != len(expected) {
		t.Fatalf("expected %d lines, got %d: %v", len(expected), len(content), content)
	}
	for i, want := range expected {
		if content[i] != want {
			t.Errorf("line %d: got %q, want %q", i, content[i], want)
		}
	}
}

func TestImportsAddGoType(t *testing.T) {
	s := NewImportSet()
	s.AddGoType(GoType{Name: "time.Time", Import: "time"})
	s.AddGoType(GoType{Name: "string", Import: ""})
	sorted := s.Sorted()
	if len(sorted) != 1 || sorted[0] != "time" {
		t.Errorf("expected [time], got %v", sorted)
	}
}

func TestImportsOnlyThirdParty(t *testing.T) {
	s := NewImportSet()
	s.Add("github.com/google/uuid")
	got := s.FormatBlock()
	if strings.Contains(got, "\n\n") {
		t.Error("should not have blank separator line with only third-party imports")
	}
	want := "import (\n\t\"github.com/google/uuid\"\n)"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

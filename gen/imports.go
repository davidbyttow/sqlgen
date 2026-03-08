package gen

import (
	"fmt"
	"sort"
	"strings"
)

// ImportSet tracks imports needed by a generated file.
type ImportSet struct {
	imports map[string]bool // import path -> exists
}

// NewImportSet creates an empty import set.
func NewImportSet() *ImportSet {
	return &ImportSet{imports: make(map[string]bool)}
}

// Add adds an import path. Empty strings are ignored.
func (s *ImportSet) Add(path string) {
	if path != "" {
		s.imports[path] = true
	}
}

// AddGoType adds the import for a GoType if needed.
func (s *ImportSet) AddGoType(gt GoType) {
	s.Add(gt.Import)
}

// Sorted returns the import paths sorted alphabetically.
func (s *ImportSet) Sorted() []string {
	paths := make([]string, 0, len(s.imports))
	for p := range s.imports {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// FormatBlock returns a formatted Go import block string.
// Groups stdlib imports separately from third-party.
func (s *ImportSet) FormatBlock() string {
	all := s.Sorted()
	if len(all) == 0 {
		return ""
	}

	var stdlib, thirdParty []string
	for _, p := range all {
		if isStdlib(p) {
			stdlib = append(stdlib, p)
		} else {
			thirdParty = append(thirdParty, p)
		}
	}

	var b strings.Builder
	b.WriteString("import (\n")
	for _, p := range stdlib {
		fmt.Fprintf(&b, "\t%q\n", p)
	}
	if len(stdlib) > 0 && len(thirdParty) > 0 {
		b.WriteString("\n")
	}
	for _, p := range thirdParty {
		fmt.Fprintf(&b, "\t%q\n", p)
	}
	b.WriteString(")")
	return b.String()
}

// isStdlib heuristically checks if an import path is a stdlib package.
func isStdlib(path string) bool {
	// Stdlib packages don't contain dots in their first path component.
	if i := strings.Index(path, "/"); i >= 0 {
		return !strings.Contains(path[:i], ".")
	}
	return !strings.Contains(path, ".")
}

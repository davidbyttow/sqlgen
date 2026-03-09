package gen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidbyttow/sqlgen/config"
	"github.com/davidbyttow/sqlgen/schema"
	"github.com/davidbyttow/sqlgen/schema/postgres"
)

// Set SQLGEN_UPDATE_GOLDEN=1 to update golden files.
var updateGolden = os.Getenv("SQLGEN_UPDATE_GOLDEN") == "1"

func TestGoldenFiles(t *testing.T) {
	t.Parallel()
	// Parse the test fixture schema.
	p := &postgres.Parser{}
	s, err := p.ParseFile("../schema/postgres/testdata/schema.sql")
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	schema.ResolveRelationships(s)

	// Generate to a temp directory.
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "models")

	cfg := &config.Config{
		Input:  config.InputConfig{Dialect: "postgres", Paths: []string{"../schema/postgres/testdata/schema.sql"}},
		Output: config.OutputConfig{Dir: outDir, Package: "models"},
		Types:  config.TypesConfig{NullType: config.NullTypeGeneric},
	}

	g := NewGenerator(cfg, s)
	if err := g.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	goldenDir := "testdata/golden"

	if updateGolden {
		// Write generated files as new golden files.
		os.MkdirAll(goldenDir, 0o755)
		entries, _ := os.ReadDir(outDir)
		for _, e := range entries {
			content, _ := os.ReadFile(filepath.Join(outDir, e.Name()))
			if err := os.WriteFile(filepath.Join(goldenDir, e.Name()), content, 0o644); err != nil {
				t.Fatalf("writing golden file: %v", err)
			}
		}
		t.Log("Golden files updated")
		return
	}

	// Compare each generated file against golden.
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	for _, e := range entries {
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			got, err := os.ReadFile(filepath.Join(outDir, name))
			if err != nil {
				t.Fatalf("reading generated: %v", err)
			}

			goldenPath := filepath.Join(goldenDir, name)
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("reading golden file %s: %v (run with SQLGEN_UPDATE_GOLDEN=1 to create)", goldenPath, err)
			}

			if string(got) != string(want) {
				// Show a useful diff.
				gotLines := strings.Split(string(got), "\n")
				wantLines := strings.Split(string(want), "\n")

				// Find first difference.
				maxLines := len(gotLines)
				if len(wantLines) > maxLines {
					maxLines = len(wantLines)
				}
				for i := 0; i < maxLines; i++ {
					var gotLine, wantLine string
					if i < len(gotLines) {
						gotLine = gotLines[i]
					}
					if i < len(wantLines) {
						wantLine = wantLines[i]
					}
					if gotLine != wantLine {
						t.Errorf("mismatch at line %d:\n  got:  %q\n  want: %q", i+1, gotLine, wantLine)
						if i+1 < maxLines {
							// Show a few more lines of context.
							for j := i + 1; j < i+4 && j < maxLines; j++ {
								if j < len(gotLines) {
									gotLine = gotLines[j]
								} else {
									gotLine = "<EOF>"
								}
								if j < len(wantLines) {
									wantLine = wantLines[j]
								} else {
									wantLine = "<EOF>"
								}
								if gotLine != wantLine {
									t.Errorf("  line %d: got=%q want=%q", j+1, gotLine, wantLine)
								}
							}
						}
						break
					}
				}
				t.Fatalf("golden file mismatch for %s (run with SQLGEN_UPDATE_GOLDEN=1 to update)", name)
			}
		})
	}

	// Also check that no golden files exist for files we didn't generate.
	goldenEntries, err := os.ReadDir(goldenDir)
	if err != nil {
		t.Fatalf("ReadDir golden: %v", err)
	}
	genSet := make(map[string]bool)
	for _, e := range entries {
		genSet[e.Name()] = true
	}
	for _, e := range goldenEntries {
		if !genSet[e.Name()] && strings.HasSuffix(e.Name(), ".go") {
			t.Errorf("stale golden file: %s (no longer generated)", e.Name())
		}
	}
}

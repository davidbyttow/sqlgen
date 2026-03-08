package gen

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/davidbyttow/sqlgen/config"
	"github.com/davidbyttow/sqlgen/schema"
	"github.com/davidbyttow/sqlgen/schema/postgres"
)

func TestEdgeCasesCompile(t *testing.T) {
	p := &postgres.Parser{}
	s, err := p.ParseFile("../schema/postgres/testdata/edge_cases.sql")
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	schema.ResolveRelationships(s)

	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "models")

	cfg := &config.Config{
		Input:  config.InputConfig{Dialect: "postgres", Paths: []string{"x"}},
		Output: config.OutputConfig{Dir: outDir, Package: "models"},
		Types:  config.TypesConfig{NullType: config.NullTypeGeneric},
	}

	g := NewGenerator(cfg, s)
	if err := g.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Set up temp module.
	goMod := `module testgen
go 1.23
require github.com/davidbyttow/sqlgen v0.0.0
replace github.com/davidbyttow/sqlgen => ` + getModuleRoot() + `
`
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0o644)

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyCmd.Env = append(os.Environ(), "GOFLAGS=")
	if out, err := tidyCmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %s\n%v", out, err)
	}

	cmd := exec.Command("go", "build", "./models/...")
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "GOFLAGS=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Dump files for debugging.
		entries, _ := os.ReadDir(outDir)
		for _, e := range entries {
			content, _ := os.ReadFile(filepath.Join(outDir, e.Name()))
			t.Logf("=== %s ===\n%s", e.Name(), content)
		}
		t.Fatalf("edge case code does not compile:\n%s\n%v", out, err)
	}
	t.Log("Edge case code compiles successfully")
}

func TestPointerNullStrategy(t *testing.T) {
	s := &schema.Schema{
		Tables: []*schema.Table{
			{
				Name: "items",
				Columns: []*schema.Column{
					{Name: "id", DBType: "integer", IsAutoIncrement: true, HasDefault: true},
					{Name: "name", DBType: "text"},
					{Name: "description", DBType: "text", IsNullable: true},
					{Name: "count", DBType: "integer", IsNullable: true},
				},
				PrimaryKey: &schema.PrimaryKey{Columns: []string{"id"}},
			},
		},
	}

	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "models")

	cfg := &config.Config{
		Input:  config.InputConfig{Dialect: "postgres", Paths: []string{"x"}},
		Output: config.OutputConfig{Dir: outDir, Package: "models"},
		Types:  config.TypesConfig{NullType: config.NullTypePointer},
	}

	g := NewGenerator(cfg, s)
	if err := g.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	goMod := `module testgen
go 1.23
require github.com/davidbyttow/sqlgen v0.0.0
replace github.com/davidbyttow/sqlgen => ` + getModuleRoot() + `
`
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0o644)

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyCmd.Env = append(os.Environ(), "GOFLAGS=")
	if out, err := tidyCmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %s\n%v", out, err)
	}

	cmd := exec.Command("go", "build", "./models/...")
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "GOFLAGS=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		entries, _ := os.ReadDir(outDir)
		for _, e := range entries {
			content, _ := os.ReadFile(filepath.Join(outDir, e.Name()))
			t.Logf("=== %s ===\n%s", e.Name(), content)
		}
		t.Fatalf("pointer null strategy does not compile:\n%s\n%v", out, err)
	}
	t.Log("Pointer null strategy compiles successfully")
}

func TestDatabaseNullStrategy(t *testing.T) {
	s := &schema.Schema{
		Tables: []*schema.Table{
			{
				Name: "items",
				Columns: []*schema.Column{
					{Name: "id", DBType: "integer", IsAutoIncrement: true, HasDefault: true},
					{Name: "name", DBType: "text"},
					{Name: "note", DBType: "text", IsNullable: true},
					{Name: "count", DBType: "integer", IsNullable: true},
					{Name: "active", DBType: "boolean", IsNullable: true},
					{Name: "created_at", DBType: "timestamp with time zone", IsNullable: true},
				},
				PrimaryKey: &schema.PrimaryKey{Columns: []string{"id"}},
			},
		},
	}

	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "models")

	cfg := &config.Config{
		Input:  config.InputConfig{Dialect: "postgres", Paths: []string{"x"}},
		Output: config.OutputConfig{Dir: outDir, Package: "models"},
		Types:  config.TypesConfig{NullType: config.NullTypeDatabase},
	}

	g := NewGenerator(cfg, s)
	if err := g.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	goMod := `module testgen
go 1.23
require github.com/davidbyttow/sqlgen v0.0.0
replace github.com/davidbyttow/sqlgen => ` + getModuleRoot() + `
`
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0o644)

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyCmd.Env = append(os.Environ(), "GOFLAGS=")
	if out, err := tidyCmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %s\n%v", out, err)
	}

	cmd := exec.Command("go", "build", "./models/...")
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "GOFLAGS=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		entries, _ := os.ReadDir(outDir)
		for _, e := range entries {
			content, _ := os.ReadFile(filepath.Join(outDir, e.Name()))
			t.Logf("=== %s ===\n%s", e.Name(), content)
		}
		t.Fatalf("database null strategy does not compile:\n%s\n%v", out, err)
	}
	t.Log("Database null strategy compiles successfully")
}

func TestSkipTable(t *testing.T) {
	s := &schema.Schema{
		Tables: []*schema.Table{
			{
				Name:       "keep",
				Columns:    []*schema.Column{{Name: "id", DBType: "integer"}},
				PrimaryKey: &schema.PrimaryKey{Columns: []string{"id"}},
			},
			{
				Name:       "skip_me",
				Columns:    []*schema.Column{{Name: "id", DBType: "integer"}},
				PrimaryKey: &schema.PrimaryKey{Columns: []string{"id"}},
			},
		},
	}

	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "models")

	cfg := &config.Config{
		Input:  config.InputConfig{Dialect: "postgres", Paths: []string{"x"}},
		Output: config.OutputConfig{Dir: outDir, Package: "models"},
		Types:  config.TypesConfig{NullType: config.NullTypeGeneric},
		Tables: map[string]config.TableConfig{
			"skip_me": {Skip: true},
		},
	}

	g := NewGenerator(cfg, s)
	if err := g.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	entries, _ := os.ReadDir(outDir)
	for _, e := range entries {
		if e.Name() == "sqlgen_skip_me_model.go" {
			t.Error("skip_me table should not have been generated")
		}
	}

	// "keep" table should exist.
	if _, err := os.Stat(filepath.Join(outDir, "sqlgen_keep_model.go")); os.IsNotExist(err) {
		t.Error("keep table should have been generated")
	}
}

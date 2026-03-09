package gen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidbyttow/sqlgen/config"
	"github.com/davidbyttow/sqlgen/schema"
	"github.com/davidbyttow/sqlgen/schema/postgres"
)

func TestGeneratorEndToEnd(t *testing.T) {
	// Parse the test fixture schema.
	p := &postgres.Parser{}
	s, err := p.ParseFile("../schema/postgres/testdata/schema.sql")
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	schema.ResolveRelationships(s)

	// Create a temp directory for output.
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

	// List generated files.
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	var files []string
	for _, e := range entries {
		files = append(files, e.Name())
	}
	t.Logf("Generated %d files: %s", len(files), strings.Join(files, ", "))

	// Verify we got files for each table + enums + dialect.
	expectedFiles := []string{
		"sqlgen_dialect.go",
		"sqlgen_enum_user_role.go",
		"sqlgen_organizations_model.go",
		"sqlgen_organizations_crud.go",
		"sqlgen_organizations_hooks.go",
		"sqlgen_organizations_where.go",
		"sqlgen_organizations_loaders.go",
		"sqlgen_users_model.go",
		"sqlgen_users_crud.go",
		"sqlgen_users_loaders.go",
		"sqlgen_posts_model.go",
		"sqlgen_posts_loaders.go",
		"sqlgen_tags_model.go",
		"sqlgen_tags_loaders.go",
		"sqlgen_post_tags_model.go",
		"sqlgen_audit_log_model.go",
		"sqlgen_audit_log_loaders.go",
		"sqlgen_categories_model.go",
		"sqlgen_categories_loaders.go",
	}

	fileSet := make(map[string]bool)
	for _, f := range files {
		fileSet[f] = true
	}
	for _, expected := range expectedFiles {
		if !fileSet[expected] {
			t.Errorf("missing expected file: %s", expected)
		}
	}

	// Create a go.mod in the temp dir so the generated code can compile.
	goMod := `module testgen

go 1.23

require github.com/davidbyttow/sqlgen v0.0.0

replace github.com/davidbyttow/sqlgen => ` + getModuleRoot() + `
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}

	// Run go mod tidy first, then go build.
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	tidyCmd.Env = append(os.Environ(), "GOFLAGS=")
	if out, err := tidyCmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed:\n%s\n%v", out, err)
	}

	// Run go build on the generated package to verify it compiles.
	cmd := exec.Command("go", "build", "./models/...")
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "GOFLAGS=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Print generated files for debugging.
		for _, f := range files {
			content, _ := os.ReadFile(filepath.Join(outDir, f))
			t.Logf("=== %s ===\n%s", f, content)
		}
		t.Fatalf("generated code does not compile:\n%s\n%v", out, err)
	}

	t.Log("Generated code compiles successfully")
}

func TestStaleFileCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "models")
	os.MkdirAll(outDir, 0o755)

	// Create a stale file.
	stalePath := filepath.Join(outDir, "sqlgen_old_table_model.go")
	os.WriteFile(stalePath, []byte("package models\n"), 0o644)

	// Also create a non-sqlgen file that should NOT be deleted.
	keepPath := filepath.Join(outDir, "custom.go")
	os.WriteFile(keepPath, []byte("package models\n"), 0o644)

	// Generate with a minimal schema.
	s := &schema.Schema{
		Tables: []*schema.Table{
			{
				Name:       "items",
				Columns:    []*schema.Column{{Name: "id", DBType: "integer"}},
				PrimaryKey: &schema.PrimaryKey{Columns: []string{"id"}},
			},
		},
	}

	cfg := &config.Config{
		Input:  config.InputConfig{Dialect: "postgres", Paths: []string{"x"}},
		Output: config.OutputConfig{Dir: outDir, Package: "models"},
		Types:  config.TypesConfig{NullType: config.NullTypeGeneric},
	}

	g := NewGenerator(cfg, s)
	if err := g.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Stale file should be gone.
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Error("stale file should have been removed")
	}

	// Custom file should still exist.
	if _, err := os.Stat(keepPath); os.IsNotExist(err) {
		t.Error("non-sqlgen file should not be removed")
	}
}

func getModuleRoot() string {
	// Walk up from gen/ to find go.mod.
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

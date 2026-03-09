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

func TestGeneratorEndToEnd(t *testing.T) {
	t.Parallel()
	p := &postgres.Parser{}
	s, err := p.ParseFile("../schema/postgres/testdata/schema.sql")
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	schema.ResolveRelationships(s)

	cfg := &config.Config{
		Input:  config.InputConfig{Dialect: "postgres", Paths: []string{"../schema/postgres/testdata/schema.sql"}},
		Output: config.OutputConfig{},
		Types:  config.TypesConfig{NullType: config.NullTypeGeneric},
	}
	g := NewGenerator(cfg, s)
	outDir := generateAndBuild(t, g, "endtoend")

	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	var files []string
	for _, e := range entries {
		files = append(files, e.Name())
	}
	t.Logf("Generated %d files: %s", len(files), strings.Join(files, ", "))

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

	t.Log("Generated code compiles successfully")
}

func TestGeneratedTestFiles(t *testing.T) {
	t.Parallel()
	p := &postgres.Parser{}
	s, err := p.ParseFile("../schema/postgres/testdata/schema.sql")
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	schema.ResolveRelationships(s)

	cfg := &config.Config{
		Input:  config.InputConfig{Dialect: "postgres", Paths: []string{"../schema/postgres/testdata/schema.sql"}},
		Output: config.OutputConfig{Tests: true},
		Types:  config.TypesConfig{NullType: config.NullTypeGeneric},
	}
	g := NewGenerator(cfg, s)
	outDir := generateAndTest(t, g, "gentests")

	entries, _ := os.ReadDir(outDir)
	var testFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "_test.go") {
			testFiles = append(testFiles, e.Name())
		}
	}
	if len(testFiles) == 0 {
		t.Fatal("no test files generated")
	}
	t.Logf("Generated %d test files: %s", len(testFiles), strings.Join(testFiles, ", "))
}

func TestStaleFileCleanup(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "models")
	os.MkdirAll(outDir, 0o755)

	stalePath := filepath.Join(outDir, "sqlgen_old_table_model.go")
	os.WriteFile(stalePath, []byte("package models\n"), 0o644)

	keepPath := filepath.Join(outDir, "custom.go")
	os.WriteFile(keepPath, []byte("package models\n"), 0o644)

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

	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Error("stale file should have been removed")
	}
	if _, err := os.Stat(keepPath); os.IsNotExist(err) {
		t.Error("non-sqlgen file should not be removed")
	}
}

func getModuleRoot() string {
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

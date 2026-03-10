package gen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/davidbyttow/sqlgen/config"
	"github.com/davidbyttow/sqlgen/schema"
	"github.com/davidbyttow/sqlgen/schema/postgres"
)

func TestEdgeCasesCompile(t *testing.T) {
	t.Parallel()
	p := &postgres.Parser{}
	s, err := p.ParseFile("../schema/postgres/testdata/edge_cases.sql")
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	schema.ResolveRelationships(s)

	cfg := &config.Config{
		Input:  config.InputConfig{Dialect: "postgres", Paths: []string{"x"}},
		Output: config.OutputConfig{},
		Types:  config.TypesConfig{NullType: config.NullTypeGeneric},
	}
	g := NewGenerator(cfg, s)
	generateAndBuild(t, g, "edgecases")
	t.Log("Edge case code compiles successfully")
}

func TestPointerNullStrategy(t *testing.T) {
	t.Parallel()
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

	cfg := &config.Config{
		Input:  config.InputConfig{Dialect: "postgres", Paths: []string{"x"}},
		Output: config.OutputConfig{},
		Types:  config.TypesConfig{NullType: config.NullTypePointer},
	}
	g := NewGenerator(cfg, s)
	generateAndBuild(t, g, "ptrnull")
	t.Log("Pointer null strategy compiles successfully")
}

func TestDatabaseNullStrategy(t *testing.T) {
	t.Parallel()
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

	cfg := &config.Config{
		Input:  config.InputConfig{Dialect: "postgres", Paths: []string{"x"}},
		Output: config.OutputConfig{},
		Types:  config.TypesConfig{NullType: config.NullTypeDatabase},
	}
	g := NewGenerator(cfg, s)
	generateAndBuild(t, g, "dbnull")
	t.Log("Database null strategy compiles successfully")
}

func TestPolymorphicCompile(t *testing.T) {
	t.Parallel()
	s := &schema.Schema{
		Tables: []*schema.Table{
			{
				Name: "users",
				Columns: []*schema.Column{
					{Name: "id", DBType: "integer", IsAutoIncrement: true, HasDefault: true},
					{Name: "name", DBType: "text"},
				},
				PrimaryKey: &schema.PrimaryKey{Columns: []string{"id"}},
			},
			{
				Name: "posts",
				Columns: []*schema.Column{
					{Name: "id", DBType: "integer", IsAutoIncrement: true, HasDefault: true},
					{Name: "title", DBType: "text"},
				},
				PrimaryKey: &schema.PrimaryKey{Columns: []string{"id"}},
			},
			{
				Name: "comments",
				Columns: []*schema.Column{
					{Name: "id", DBType: "integer", IsAutoIncrement: true, HasDefault: true},
					{Name: "body", DBType: "text"},
					{Name: "commentable_type", DBType: "text"},
					{Name: "commentable_id", DBType: "integer"},
				},
				PrimaryKey: &schema.PrimaryKey{Columns: []string{"id"}},
			},
		},
	}

	schema.ResolveRelationships(s)
	schema.ResolvePolymorphic(s, []schema.PolymorphicDef{
		{
			Table:      "comments",
			TypeColumn: "commentable_type",
			IDColumn:   "commentable_id",
			Targets:    map[string]string{"User": "users", "Post": "posts"},
		},
	})

	cfg := &config.Config{
		Input:  config.InputConfig{Dialect: "postgres", Paths: []string{"x"}},
		Output: config.OutputConfig{},
		Types:  config.TypesConfig{NullType: config.NullTypeGeneric},
	}
	g := NewGenerator(cfg, s)
	generateAndBuild(t, g, "polymorphic")
	t.Log("Polymorphic code compiles successfully")
}

func TestSkipTable(t *testing.T) {
	t.Parallel()
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

	cfg := &config.Config{
		Input:  config.InputConfig{Dialect: "postgres", Paths: []string{"x"}},
		Output: config.OutputConfig{},
		Types:  config.TypesConfig{NullType: config.NullTypeGeneric},
		Tables: map[string]config.TableConfig{
			"skip_me": {Skip: true},
		},
	}
	g := NewGenerator(cfg, s)
	outDir := generateAndBuild(t, g, "skiptable")

	entries, _ := os.ReadDir(outDir)
	for _, e := range entries {
		if e.Name() == "sqlgen_skip_me_model.go" {
			t.Error("skip_me table should not have been generated")
		}
	}
	if _, err := os.Stat(filepath.Join(outDir, "sqlgen_keep_model.go")); os.IsNotExist(err) {
		t.Error("keep table should have been generated")
	}
}

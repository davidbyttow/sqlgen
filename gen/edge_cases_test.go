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

func TestColumnTypeReplacement(t *testing.T) {
	t.Parallel()
	s := &schema.Schema{
		Tables: []*schema.Table{
			{
				Name: "users",
				Columns: []*schema.Column{
					{Name: "id", DBType: "uuid"},
					{Name: "name", DBType: "text"},
					{Name: "metadata", DBType: "jsonb"},
				},
				PrimaryKey: &schema.PrimaryKey{Columns: []string{"id"}},
			},
			{
				Name: "posts",
				Columns: []*schema.Column{
					{Name: "id", DBType: "uuid"},
					{Name: "title", DBType: "text"},
					{Name: "metadata", DBType: "jsonb"},
				},
				PrimaryKey: &schema.PrimaryKey{Columns: []string{"id"}},
			},
		},
	}

	cfg := &config.Config{
		Input:  config.InputConfig{Dialect: "postgres", Paths: []string{"x"}},
		Output: config.OutputConfig{},
		Types: config.TypesConfig{
			NullType: config.NullTypeGeneric,
			// Exact match: users.metadata -> map[string]any
			// Wildcard: *.id -> string (override uuid default which is also string, but tests the path)
			ColumnReplacements: map[string]string{
				"users.metadata": "map[string]any",
				"*.id":           "string",
			},
		},
	}
	g := NewGenerator(cfg, s)
	outDir := generateAndBuild(t, g, "colrepl")

	// Verify that the users model uses map[string]any for metadata (exact match).
	usersModel, err := os.ReadFile(filepath.Join(outDir, "sqlgen_users_model.go"))
	if err != nil {
		t.Fatalf("reading users model: %v", err)
	}
	content := string(usersModel)
	if !contains(content, "map[string]any") {
		t.Error("expected users model to contain map[string]any for metadata column")
	}

	// Verify posts model keeps json.RawMessage for metadata (no override for posts.metadata).
	postsModel, err := os.ReadFile(filepath.Join(outDir, "sqlgen_posts_model.go"))
	if err != nil {
		t.Fatalf("reading posts model: %v", err)
	}
	postsContent := string(postsModel)
	if !contains(postsContent, "json.RawMessage") {
		t.Error("expected posts model to keep json.RawMessage for metadata (no override)")
	}
	t.Log("Column type replacement compiles successfully")
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && strings.Contains(s, substr)
}

func TestFactoryCompile(t *testing.T) {
	t.Parallel()
	s := &schema.Schema{
		Tables: []*schema.Table{
			{
				Name: "users",
				Columns: []*schema.Column{
					{Name: "id", DBType: "integer", IsAutoIncrement: true, HasDefault: true},
					{Name: "name", DBType: "text"},
					{Name: "email", DBType: "text"},
					{Name: "bio", DBType: "text", IsNullable: true},
					{Name: "age", DBType: "integer", IsNullable: true},
					{Name: "active", DBType: "boolean"},
					{Name: "score", DBType: "double precision"},
					{Name: "metadata", DBType: "jsonb"},
					{Name: "created_at", DBType: "timestamp with time zone"},
				},
				PrimaryKey: &schema.PrimaryKey{Columns: []string{"id"}},
			},
			{
				Name: "posts",
				Columns: []*schema.Column{
					{Name: "id", DBType: "bigint", IsAutoIncrement: true, HasDefault: true},
					{Name: "title", DBType: "text"},
					{Name: "body", DBType: "text"},
					{Name: "user_id", DBType: "integer"},
				},
				PrimaryKey: &schema.PrimaryKey{Columns: []string{"id"}},
				ForeignKeys: []*schema.ForeignKey{
					{Name: "posts_user_id_fkey", Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}},
				},
			},
		},
		Enums: []*schema.Enum{
			{Name: "status", Values: []string{"active", "inactive", "pending"}},
		},
	}
	schema.ResolveRelationships(s)

	cfg := &config.Config{
		Input:  config.InputConfig{Dialect: "postgres", Paths: []string{"x"}},
		Output: config.OutputConfig{Factories: true},
		Types:  config.TypesConfig{NullType: config.NullTypeGeneric},
	}
	g := NewGenerator(cfg, s)
	outDir := generateAndBuild(t, g, "factories")

	// Verify factory files exist.
	if _, err := os.Stat(filepath.Join(outDir, "sqlgen_users_factory.go")); os.IsNotExist(err) {
		t.Error("users factory file not generated")
	}
	if _, err := os.Stat(filepath.Join(outDir, "sqlgen_posts_factory.go")); os.IsNotExist(err) {
		t.Error("posts factory file not generated")
	}

	// Verify factory content has NewUser and InsertUser.
	content, _ := os.ReadFile(filepath.Join(outDir, "sqlgen_users_factory.go"))
	if !contains(string(content), "func NewUser(") {
		t.Error("expected NewUser function")
	}
	if !contains(string(content), "func InsertUser(") {
		t.Error("expected InsertUser function")
	}
	// Verify auto-increment ID is not in the factory defaults.
	if contains(string(content), "Id:") {
		t.Error("auto-increment ID should not be in factory defaults")
	}
	t.Log("Factory code compiles successfully")
}

func TestFactoryWithEnumsCompile(t *testing.T) {
	t.Parallel()
	s := &schema.Schema{
		Tables: []*schema.Table{
			{
				Name: "items",
				Columns: []*schema.Column{
					{Name: "id", DBType: "integer", IsAutoIncrement: true, HasDefault: true},
					{Name: "status", DBType: "item_status", EnumName: "item_status"},
				},
				PrimaryKey: &schema.PrimaryKey{Columns: []string{"id"}},
			},
		},
		Enums: []*schema.Enum{
			{Name: "item_status", Values: []string{"active", "inactive"}},
		},
	}

	cfg := &config.Config{
		Input:  config.InputConfig{Dialect: "postgres", Paths: []string{"x"}},
		Output: config.OutputConfig{Factories: true},
		Types:  config.TypesConfig{NullType: config.NullTypeGeneric},
	}
	g := NewGenerator(cfg, s)
	outDir := generateAndBuild(t, g, "factenum")

	content, _ := os.ReadFile(filepath.Join(outDir, "sqlgen_items_factory.go"))
	if !contains(string(content), `ItemStatus("active")`) {
		t.Error("expected enum factory to use first enum value")
	}
	t.Log("Factory with enums compiles successfully")
}

func TestFactoryNotGeneratedWhenDisabled(t *testing.T) {
	t.Parallel()
	s := &schema.Schema{
		Tables: []*schema.Table{
			{
				Name:       "things",
				Columns:    []*schema.Column{{Name: "id", DBType: "integer"}},
				PrimaryKey: &schema.PrimaryKey{Columns: []string{"id"}},
			},
		},
	}

	cfg := &config.Config{
		Input:  config.InputConfig{Dialect: "postgres", Paths: []string{"x"}},
		Output: config.OutputConfig{Factories: false},
		Types:  config.TypesConfig{NullType: config.NullTypeGeneric},
	}
	g := NewGenerator(cfg, s)
	outDir := generateAndBuild(t, g, "nofactory")

	if _, err := os.Stat(filepath.Join(outDir, "sqlgen_things_factory.go")); !os.IsNotExist(err) {
		t.Error("factory file should not be generated when disabled")
	}
}

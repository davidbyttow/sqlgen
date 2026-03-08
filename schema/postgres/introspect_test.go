package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/davidbyttow/sqlgen/schema"
)

func TestMapPgAction(t *testing.T) {
	tests := []struct {
		code string
		want schema.Action
	}{
		{"a", schema.ActionNoAction},
		{"r", schema.ActionRestrict},
		{"c", schema.ActionCascade},
		{"n", schema.ActionSetNull},
		{"d", schema.ActionSetDefault},
		{"", schema.ActionNone},
		{"x", schema.ActionNone},
	}
	for _, tt := range tests {
		got := mapPgAction(tt.code)
		if got != tt.want {
			t.Errorf("mapPgAction(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

// TestIntrospectLive runs against a real Postgres database.
// Set SQLGEN_TEST_DSN to enable (e.g., "postgres://localhost:5432/sqlgen_test").
func TestIntrospectLive(t *testing.T) {
	dsn := os.Getenv("SQLGEN_TEST_DSN")
	if dsn == "" {
		t.Skip("SQLGEN_TEST_DSN not set, skipping live introspection test")
	}

	ctx := context.Background()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer db.Close()

	// Create a temp schema to isolate the test.
	schemaName := "sqlgen_introspect_test"
	mustExec(t, db, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName))
	mustExec(t, db, fmt.Sprintf("CREATE SCHEMA %s", schemaName))
	mustExec(t, db, fmt.Sprintf("SET search_path TO %s", schemaName))
	t.Cleanup(func() {
		mustExec(t, db, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName))
	})

	// Set up test schema.
	ddl := `
		CREATE TYPE user_role AS ENUM ('admin', 'member', 'guest');

		CREATE TABLE organizations (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);

		CREATE TABLE users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			org_id UUID NOT NULL REFERENCES organizations(id),
			email TEXT NOT NULL,
			role user_role NOT NULL DEFAULT 'member',
			name TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ
		);

		CREATE UNIQUE INDEX idx_users_email ON users(email);

		CREATE TABLE posts (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			author_id UUID NOT NULL REFERENCES users(id),
			title TEXT NOT NULL,
			body TEXT NOT NULL,
			published BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);

		CREATE TABLE tags (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE
		);

		CREATE TABLE post_tags (
			post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
			tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
			PRIMARY KEY (post_id, tag_id)
		);

		CREATE VIEW published_posts AS
			SELECT id, title, author_id FROM posts WHERE published = true;
	`
	mustExec(t, db, ddl)

	s, err := IntrospectDB(ctx, db)
	if err != nil {
		t.Fatalf("IntrospectDB error: %v", err)
	}

	// Check enums.
	if len(s.Enums) != 1 {
		t.Fatalf("enums: got %d, want 1", len(s.Enums))
	}
	e := s.Enums[0]
	if e.Name != "user_role" {
		t.Errorf("enum name = %q, want user_role", e.Name)
	}
	if len(e.Values) != 3 {
		t.Errorf("enum values: got %d, want 3", len(e.Values))
	}

	// Check tables.
	if len(s.Tables) != 5 {
		t.Fatalf("tables: got %d, want 5", len(s.Tables))
	}

	tableNames := map[string]*schema.Table{}
	for _, tbl := range s.Tables {
		tableNames[tbl.Name] = tbl
	}

	// Check organizations table.
	org := tableNames["organizations"]
	if org == nil {
		t.Fatal("missing organizations table")
	}
	if len(org.Columns) != 3 {
		t.Errorf("organizations columns: got %d, want 3", len(org.Columns))
	}
	if org.PrimaryKey == nil || len(org.PrimaryKey.Columns) != 1 || org.PrimaryKey.Columns[0] != "id" {
		t.Errorf("organizations PK: %+v", org.PrimaryKey)
	}

	// Check users table.
	users := tableNames["users"]
	if users == nil {
		t.Fatal("missing users table")
	}
	if len(users.Columns) != 7 {
		t.Errorf("users columns: got %d, want 7", len(users.Columns))
	}
	if len(users.ForeignKeys) != 1 {
		t.Errorf("users FKs: got %d, want 1", len(users.ForeignKeys))
	}

	// Check FK details.
	if len(users.ForeignKeys) > 0 {
		fk := users.ForeignKeys[0]
		if fk.RefTable != "organizations" {
			t.Errorf("users FK ref_table = %q, want organizations", fk.RefTable)
		}
		if len(fk.Columns) != 1 || fk.Columns[0] != "org_id" {
			t.Errorf("users FK columns = %v, want [org_id]", fk.Columns)
		}
	}

	// Check enum column.
	roleCol := users.FindColumn("role")
	if roleCol == nil {
		t.Fatal("missing role column")
	}
	if roleCol.EnumName != "user_role" {
		t.Errorf("role enum = %q, want user_role", roleCol.EnumName)
	}

	// Check nullable column.
	nameCol := users.FindColumn("name")
	if nameCol == nil {
		t.Fatal("missing name column")
	}
	if !nameCol.IsNullable {
		t.Error("name column should be nullable")
	}

	// Check unique index.
	if len(users.Unique) == 0 {
		t.Error("users should have unique constraints")
	}

	// Check tags table (serial PK).
	tags := tableNames["tags"]
	if tags == nil {
		t.Fatal("missing tags table")
	}
	idCol := tags.FindColumn("id")
	if idCol == nil {
		t.Fatal("missing tags.id column")
	}
	if !idCol.IsAutoIncrement {
		t.Error("tags.id should be auto-increment")
	}

	// Check post_tags (composite PK).
	pt := tableNames["post_tags"]
	if pt == nil {
		t.Fatal("missing post_tags table")
	}
	if pt.PrimaryKey == nil || len(pt.PrimaryKey.Columns) != 2 {
		t.Errorf("post_tags PK columns: got %v", pt.PrimaryKey)
	}
	if len(pt.ForeignKeys) != 2 {
		t.Errorf("post_tags FKs: got %d, want 2", len(pt.ForeignKeys))
	}

	// Check cascade on delete.
	for _, fk := range pt.ForeignKeys {
		if fk.OnDelete != schema.ActionCascade {
			t.Errorf("post_tags FK %s on_delete = %q, want CASCADE", fk.Name, fk.OnDelete)
		}
	}

	// Check views.
	if len(s.Views) != 1 {
		t.Fatalf("views: got %d, want 1", len(s.Views))
	}
	v := s.Views[0]
	if v.Name != "published_posts" {
		t.Errorf("view name = %q, want published_posts", v.Name)
	}
	if len(v.Columns) != 3 {
		t.Errorf("view columns: got %d, want 3", len(v.Columns))
	}

	// Verify relationship resolution works with introspected schema.
	schema.ResolveRelationships(s)
	if len(users.Relationships) == 0 {
		t.Error("expected relationships on users after resolution")
	}
}

func mustExec(t *testing.T, db *sql.DB, query string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), query); err != nil {
		t.Fatalf("exec %q: %v", query[:min(len(query), 60)], err)
	}
}

package postgres

import (
	"testing"

	"github.com/davidbyttow/sqlgen/schema"
)

func mustParse(t *testing.T, sql string) *schema.Schema {
	t.Helper()
	p := &Parser{}
	s, err := p.ParseString(sql)
	if err != nil {
		t.Fatalf("ParseString() error: %v", err)
	}
	return s
}

func findTable(s *schema.Schema, name string) *schema.Table {
	for _, t := range s.Tables {
		if t.Name == name {
			return t
		}
	}
	return nil
}

func findEnum(s *schema.Schema, name string) *schema.Enum {
	for _, e := range s.Enums {
		if e.Name == name {
			return e
		}
	}
	return nil
}

func TestParseCreateTable(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email TEXT NOT NULL UNIQUE,
			name TEXT,
			age INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`)

	if len(s.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(s.Tables))
	}

	users := s.Tables[0]
	if users.Name != "users" {
		t.Errorf("table name = %q, want users", users.Name)
	}

	if users.PrimaryKey == nil {
		t.Fatal("expected primary key")
	}
	if len(users.PrimaryKey.Columns) != 1 || users.PrimaryKey.Columns[0] != "id" {
		t.Errorf("PK columns = %v, want [id]", users.PrimaryKey.Columns)
	}

	// Check columns.
	tests := []struct {
		name       string
		dbType     string
		nullable   bool
		hasDefault bool
	}{
		{"id", "uuid", false, true},
		{"email", "text", false, false},
		{"name", "text", true, false},
		{"age", "integer", false, true},
		{"created_at", "timestamp with time zone", false, true},
	}

	if len(users.Columns) != len(tests) {
		t.Fatalf("expected %d columns, got %d", len(tests), len(users.Columns))
	}
	for i, tt := range tests {
		col := users.Columns[i]
		if col.Name != tt.name {
			t.Errorf("col[%d].Name = %q, want %q", i, col.Name, tt.name)
		}
		if col.DBType != tt.dbType {
			t.Errorf("col %q: DBType = %q, want %q", col.Name, col.DBType, tt.dbType)
		}
		if col.IsNullable != tt.nullable {
			t.Errorf("col %q: IsNullable = %v, want %v", col.Name, col.IsNullable, tt.nullable)
		}
		if col.HasDefault != tt.hasDefault {
			t.Errorf("col %q: HasDefault = %v, want %v", col.Name, col.HasDefault, tt.hasDefault)
		}
	}

	// Check unique constraint on email.
	if len(users.Unique) != 1 {
		t.Fatalf("expected 1 unique constraint, got %d", len(users.Unique))
	}
	if users.Unique[0].Columns[0] != "email" {
		t.Errorf("unique constraint on %v, want [email]", users.Unique[0].Columns)
	}
}

func TestParseSerialTypes(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE items (
			id SERIAL PRIMARY KEY,
			big_id BIGSERIAL NOT NULL,
			small_id SMALLSERIAL
		);
	`)

	items := s.Tables[0]
	tests := []struct {
		name    string
		dbType  string
		autoInc bool
	}{
		{"id", "integer", true},
		{"big_id", "bigint", true},
		{"small_id", "smallint", true},
	}

	for i, tt := range tests {
		col := items.Columns[i]
		if col.DBType != tt.dbType {
			t.Errorf("col %q: DBType = %q, want %q", col.Name, col.DBType, tt.dbType)
		}
		if col.IsAutoIncrement != tt.autoInc {
			t.Errorf("col %q: IsAutoIncrement = %v, want %v", col.Name, col.IsAutoIncrement, tt.autoInc)
		}
		if !col.HasDefault {
			t.Errorf("col %q: HasDefault should be true for serial", col.Name)
		}
	}
}

func TestParseArrayTypes(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE data (
			id INTEGER PRIMARY KEY,
			tags TEXT[] NOT NULL,
			matrix INTEGER[][]
		);
	`)

	data := s.Tables[0]
	tags := data.FindColumn("tags")
	if !tags.IsArray || tags.ArrayDims != 1 {
		t.Errorf("tags: IsArray=%v ArrayDims=%d, want true/1", tags.IsArray, tags.ArrayDims)
	}

	matrix := data.FindColumn("matrix")
	if !matrix.IsArray || matrix.ArrayDims != 2 {
		t.Errorf("matrix: IsArray=%v ArrayDims=%d, want true/2", matrix.IsArray, matrix.ArrayDims)
	}
}

func TestParseEnum(t *testing.T) {
	s := mustParse(t, `
		CREATE TYPE mood AS ENUM ('happy', 'sad', 'neutral');
		CREATE TYPE public.status AS ENUM ('active', 'inactive');
	`)

	if len(s.Enums) != 2 {
		t.Fatalf("expected 2 enums, got %d", len(s.Enums))
	}

	mood := findEnum(s, "mood")
	if mood == nil {
		t.Fatal("enum 'mood' not found")
	}
	if len(mood.Values) != 3 {
		t.Errorf("mood values = %v, want 3 values", mood.Values)
	}
	if mood.Values[0] != "happy" || mood.Values[1] != "sad" || mood.Values[2] != "neutral" {
		t.Errorf("mood values = %v", mood.Values)
	}

	status := findEnum(s, "status")
	if status == nil {
		t.Fatal("enum 'status' not found")
	}
	if status.Schema != "public" {
		t.Errorf("status schema = %q, want public", status.Schema)
	}
}

func TestParseEnumColumnReference(t *testing.T) {
	s := mustParse(t, `
		CREATE TYPE user_role AS ENUM ('admin', 'member');
		CREATE TABLE users (
			id SERIAL PRIMARY KEY,
			role user_role NOT NULL DEFAULT 'member'
		);
	`)

	users := findTable(s, "users")
	role := users.FindColumn("role")
	if role.EnumName != "user_role" {
		t.Errorf("role.EnumName = %q, want user_role", role.EnumName)
	}
}

func TestParseForeignKey(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE orgs (id UUID PRIMARY KEY);
		CREATE TABLE users (
			id UUID PRIMARY KEY,
			org_id UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE
		);
	`)

	users := findTable(s, "users")
	if len(users.ForeignKeys) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(users.ForeignKeys))
	}

	fk := users.ForeignKeys[0]
	if fk.RefTable != "orgs" {
		t.Errorf("FK RefTable = %q, want orgs", fk.RefTable)
	}
	if len(fk.Columns) != 1 || fk.Columns[0] != "org_id" {
		t.Errorf("FK Columns = %v, want [org_id]", fk.Columns)
	}
	if fk.OnDelete != schema.ActionCascade {
		t.Errorf("FK OnDelete = %q, want CASCADE", fk.OnDelete)
	}
}

func TestParseCompositePrimaryKey(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE post_tags (
			post_id UUID NOT NULL,
			tag_id INTEGER NOT NULL,
			PRIMARY KEY (post_id, tag_id)
		);
	`)

	pt := s.Tables[0]
	if pt.PrimaryKey == nil {
		t.Fatal("expected primary key")
	}
	if len(pt.PrimaryKey.Columns) != 2 {
		t.Fatalf("PK columns = %v, want 2 columns", pt.PrimaryKey.Columns)
	}
	if pt.PrimaryKey.Columns[0] != "post_id" || pt.PrimaryKey.Columns[1] != "tag_id" {
		t.Errorf("PK columns = %v, want [post_id, tag_id]", pt.PrimaryKey.Columns)
	}
}

func TestParseAlterTableAddConstraint(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE orgs (id UUID PRIMARY KEY);
		CREATE TABLE users (id UUID PRIMARY KEY, org_id UUID NOT NULL);
		ALTER TABLE users ADD CONSTRAINT fk_org FOREIGN KEY (org_id) REFERENCES orgs(id);
	`)

	users := findTable(s, "users")
	if len(users.ForeignKeys) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(users.ForeignKeys))
	}
	fk := users.ForeignKeys[0]
	if fk.Name != "fk_org" {
		t.Errorf("FK name = %q, want fk_org", fk.Name)
	}
	if fk.RefTable != "orgs" {
		t.Errorf("FK RefTable = %q, want orgs", fk.RefTable)
	}
}

func TestParseView(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE posts (id UUID PRIMARY KEY, title TEXT, published BOOLEAN);
		CREATE VIEW published_posts AS SELECT id, title FROM posts WHERE published = true;
	`)

	if len(s.Views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(s.Views))
	}
	v := s.Views[0]
	if v.Name != "published_posts" {
		t.Errorf("view name = %q, want published_posts", v.Name)
	}
	if v.IsMaterialized {
		t.Error("view should not be materialized")
	}
	if v.Query == "" {
		t.Error("view query should not be empty")
	}
}

func TestParseMaterializedView(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE posts (id UUID PRIMARY KEY, title TEXT, published BOOLEAN);
		CREATE MATERIALIZED VIEW popular_posts AS SELECT id, title FROM posts WHERE published = true;
	`)

	if len(s.Views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(s.Views))
	}
	v := s.Views[0]
	if v.Name != "popular_posts" {
		t.Errorf("view name = %q, want popular_posts", v.Name)
	}
	if !v.IsMaterialized {
		t.Error("view should be materialized")
	}
}

func TestParseSelfReferencingFK(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE categories (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			parent_id INTEGER REFERENCES categories(id)
		);
	`)

	cats := findTable(s, "categories")
	if len(cats.ForeignKeys) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(cats.ForeignKeys))
	}
	fk := cats.ForeignKeys[0]
	if fk.RefTable != "categories" {
		t.Errorf("self-ref FK RefTable = %q, want categories", fk.RefTable)
	}
}

func TestParseCreateIndex(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE users (id UUID PRIMARY KEY, email TEXT, org_id UUID);
		CREATE UNIQUE INDEX idx_email_org ON users (email, org_id);
		CREATE INDEX idx_org ON users (org_id);
	`)

	users := findTable(s, "users")
	if len(users.Indexes) != 2 {
		t.Fatalf("expected 2 indexes, got %d", len(users.Indexes))
	}

	idx0 := users.Indexes[0]
	if idx0.Name != "idx_email_org" || !idx0.Unique {
		t.Errorf("index 0: name=%q unique=%v", idx0.Name, idx0.Unique)
	}
	if len(idx0.Columns) != 2 {
		t.Errorf("index 0 columns = %v, want 2", idx0.Columns)
	}

	idx1 := users.Indexes[1]
	if idx1.Unique {
		t.Error("index 1 should not be unique")
	}
}

func TestParseSchemaQualifiedTable(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE public.users (
			id UUID PRIMARY KEY,
			name TEXT
		);
	`)

	users := s.Tables[0]
	if users.Schema != "public" {
		t.Errorf("schema = %q, want public", users.Schema)
	}
	if users.Name != "users" {
		t.Errorf("name = %q, want users", users.Name)
	}
}

func TestParseMultipleStatements(t *testing.T) {
	s := mustParse(t, `
		CREATE TYPE mood AS ENUM ('happy', 'sad');
		CREATE TABLE a (id SERIAL PRIMARY KEY);
		CREATE TABLE b (id SERIAL PRIMARY KEY, a_id INTEGER REFERENCES a(id));
		CREATE VIEW v AS SELECT id FROM a;
	`)

	if len(s.Tables) != 2 {
		t.Errorf("tables = %d, want 2", len(s.Tables))
	}
	if len(s.Enums) != 1 {
		t.Errorf("enums = %d, want 1", len(s.Enums))
	}
	if len(s.Views) != 1 {
		t.Errorf("views = %d, want 1", len(s.Views))
	}
}

func TestParseFile(t *testing.T) {
	p := &Parser{}
	s, err := p.ParseFile("testdata/schema.sql")
	if err != nil {
		t.Fatalf("ParseFile() error: %v", err)
	}

	// Check counts from our test fixture.
	if len(s.Tables) < 6 {
		t.Errorf("expected at least 6 tables, got %d", len(s.Tables))
	}
	if len(s.Enums) != 1 {
		t.Errorf("expected 1 enum, got %d", len(s.Enums))
	}
	if len(s.Views) != 1 {
		t.Errorf("expected 1 view, got %d", len(s.Views))
	}

	// Check that ALTER TABLE FK was applied.
	auditLog := findTable(s, "audit_log")
	if auditLog == nil {
		t.Fatal("audit_log table not found")
	}
	if len(auditLog.ForeignKeys) != 1 {
		t.Errorf("audit_log FKs = %d, want 1", len(auditLog.ForeignKeys))
	}

	// Check unique index was recorded.
	users := findTable(s, "users")
	if users == nil {
		t.Fatal("users table not found")
	}
	if len(users.Indexes) != 1 {
		t.Errorf("users indexes = %d, want 1", len(users.Indexes))
	}
}

func TestParseTypeNormalization(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE types_test (
			a INTEGER PRIMARY KEY,
			b VARCHAR(255) NOT NULL,
			c BOOLEAN,
			d TIMESTAMPTZ,
			e DOUBLE PRECISION,
			f NUMERIC(10, 2),
			g JSONB,
			h BYTEA
		);
	`)

	table := s.Tables[0]
	tests := []struct {
		col    string
		dbType string
	}{
		{"a", "integer"},
		{"b", "character varying"},
		{"c", "boolean"},
		{"d", "timestamp with time zone"},
		{"e", "double precision"},
		{"f", "numeric"},
		{"g", "jsonb"},
		{"h", "bytea"},
	}

	for _, tt := range tests {
		col := table.FindColumn(tt.col)
		if col == nil {
			t.Errorf("column %q not found", tt.col)
			continue
		}
		if col.DBType != tt.dbType {
			t.Errorf("col %q: DBType = %q, want %q", tt.col, col.DBType, tt.dbType)
		}
	}
}

func TestParseForeignKeyActions(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE parent (id SERIAL PRIMARY KEY);
		CREATE TABLE child (
			id SERIAL PRIMARY KEY,
			parent_id INTEGER REFERENCES parent(id) ON DELETE CASCADE ON UPDATE SET NULL
		);
	`)

	child := findTable(s, "child")
	if child == nil {
		t.Fatal("child table not found")
	}
	if len(child.ForeignKeys) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(child.ForeignKeys))
	}

	fk := child.ForeignKeys[0]
	if fk.RefTable != "parent" {
		t.Errorf("FK RefTable = %q, want parent", fk.RefTable)
	}
	if fk.OnDelete != schema.ActionCascade {
		t.Errorf("FK OnDelete = %q, want CASCADE", fk.OnDelete)
	}
	if fk.OnUpdate != schema.ActionSetNull {
		t.Errorf("FK OnUpdate = %q, want SET NULL", fk.OnUpdate)
	}
}

func TestParseCheckConstraintIgnored(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE products (
			id SERIAL PRIMARY KEY,
			price NUMERIC NOT NULL CHECK (price > 0),
			name TEXT NOT NULL,
			CONSTRAINT valid_name CHECK (length(name) > 0)
		);
	`)

	products := findTable(s, "products")
	if products == nil {
		t.Fatal("products table not found")
	}
	if len(products.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(products.Columns))
	}
}

func TestParseDefaultExpressions(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE defaults_test (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			counter INTEGER NOT NULL DEFAULT 0,
			label TEXT NOT NULL DEFAULT 'unnamed',
			active BOOLEAN NOT NULL DEFAULT true,
			data JSONB NOT NULL DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`)

	table := findTable(s, "defaults_test")
	if table == nil {
		t.Fatal("defaults_test table not found")
	}
	if len(table.Columns) != 6 {
		t.Fatalf("expected 6 columns, got %d", len(table.Columns))
	}

	for _, col := range table.Columns {
		if !col.HasDefault {
			t.Errorf("col %q: HasDefault = false, want true", col.Name)
		}
	}
}

func TestParseIdentityColumns(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE identity_test (
			id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			seq INTEGER GENERATED BY DEFAULT AS IDENTITY
		);
	`)

	table := findTable(s, "identity_test")
	if table == nil {
		t.Fatal("identity_test table not found")
	}
	if len(table.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(table.Columns))
	}

	for _, col := range table.Columns {
		if !col.IsAutoIncrement {
			t.Errorf("col %q: IsAutoIncrement = false, want true", col.Name)
		}
		if !col.HasDefault {
			t.Errorf("col %q: HasDefault = false, want true", col.Name)
		}
	}
}

func TestParseMultipleFKsSameTable(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE users (id UUID PRIMARY KEY);
		CREATE TABLE messages (
			id UUID PRIMARY KEY,
			sender_id UUID NOT NULL REFERENCES users(id),
			recipient_id UUID NOT NULL REFERENCES users(id)
		);
	`)

	messages := findTable(s, "messages")
	if messages == nil {
		t.Fatal("messages table not found")
	}
	if len(messages.ForeignKeys) != 2 {
		t.Fatalf("expected 2 FKs, got %d", len(messages.ForeignKeys))
	}

	for _, fk := range messages.ForeignKeys {
		if fk.RefTable != "users" {
			t.Errorf("FK RefTable = %q, want users", fk.RefTable)
		}
	}

	if messages.ForeignKeys[0].Columns[0] != "sender_id" {
		t.Errorf("FK[0] column = %q, want sender_id", messages.ForeignKeys[0].Columns[0])
	}
	if messages.ForeignKeys[1].Columns[0] != "recipient_id" {
		t.Errorf("FK[1] column = %q, want recipient_id", messages.ForeignKeys[1].Columns[0])
	}
}

func TestParseCircularFKs(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE a (id SERIAL PRIMARY KEY, b_id INTEGER);
		CREATE TABLE b (id SERIAL PRIMARY KEY, a_id INTEGER REFERENCES a(id));
		ALTER TABLE a ADD CONSTRAINT fk_b FOREIGN KEY (b_id) REFERENCES b(id);
	`)

	tableA := findTable(s, "a")
	if tableA == nil {
		t.Fatal("table a not found")
	}
	tableB := findTable(s, "b")
	if tableB == nil {
		t.Fatal("table b not found")
	}

	if len(tableA.ForeignKeys) != 1 {
		t.Fatalf("table a: expected 1 FK, got %d", len(tableA.ForeignKeys))
	}
	if tableA.ForeignKeys[0].RefTable != "b" {
		t.Errorf("table a FK RefTable = %q, want b", tableA.ForeignKeys[0].RefTable)
	}

	if len(tableB.ForeignKeys) != 1 {
		t.Fatalf("table b: expected 1 FK, got %d", len(tableB.ForeignKeys))
	}
	if tableB.ForeignKeys[0].RefTable != "a" {
		t.Errorf("table b FK RefTable = %q, want a", tableB.ForeignKeys[0].RefTable)
	}
}

func TestParseQuotedIdentifiers(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE "My Table" (
			"Column One" SERIAL PRIMARY KEY,
			"select" TEXT NOT NULL
		);
	`)

	table := findTable(s, "My Table")
	if table == nil {
		t.Fatal(`table "My Table" not found`)
	}

	if len(table.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(table.Columns))
	}
	if table.Columns[0].Name != "Column One" {
		t.Errorf("col[0].Name = %q, want %q", table.Columns[0].Name, "Column One")
	}
	if table.Columns[1].Name != "select" {
		t.Errorf("col[1].Name = %q, want %q", table.Columns[1].Name, "select")
	}
}

func TestParseCompositeFK(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE parent (
			a INTEGER,
			b INTEGER,
			PRIMARY KEY (a, b)
		);
		CREATE TABLE child (
			id SERIAL PRIMARY KEY,
			parent_a INTEGER NOT NULL,
			parent_b INTEGER NOT NULL,
			FOREIGN KEY (parent_a, parent_b) REFERENCES parent(a, b)
		);
	`)

	child := findTable(s, "child")
	if child == nil {
		t.Fatal("child table not found")
	}
	if len(child.ForeignKeys) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(child.ForeignKeys))
	}

	fk := child.ForeignKeys[0]
	if len(fk.Columns) != 2 || fk.Columns[0] != "parent_a" || fk.Columns[1] != "parent_b" {
		t.Errorf("FK Columns = %v, want [parent_a parent_b]", fk.Columns)
	}
	if len(fk.RefColumns) != 2 || fk.RefColumns[0] != "a" || fk.RefColumns[1] != "b" {
		t.Errorf("FK RefColumns = %v, want [a b]", fk.RefColumns)
	}
}

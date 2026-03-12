package sqlite

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

func TestParseCreateTable(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			name TEXT,
			age INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
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

	tests := []struct {
		name       string
		dbType     string
		nullable   bool
		hasDefault bool
	}{
		{"id", "integer", false, true},
		{"email", "text", false, false},
		{"name", "text", true, false},
		{"age", "integer", false, true},
		{"created_at", "datetime", false, true},
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

func TestParseIntegerPrimaryKeyAutoIncrement(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE items (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL
		);
	`)

	items := s.Tables[0]
	idCol := items.FindColumn("id")
	if !idCol.IsAutoIncrement {
		t.Error("INTEGER PRIMARY KEY should be auto-increment")
	}
	if !idCol.HasDefault {
		t.Error("INTEGER PRIMARY KEY should have default")
	}
	if idCol.IsNullable {
		t.Error("PRIMARY KEY column should not be nullable")
	}
}

func TestParseAutoincrement(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL
		);
	`)

	items := s.Tables[0]
	idCol := items.FindColumn("id")
	if !idCol.IsAutoIncrement {
		t.Error("AUTOINCREMENT column should be auto-increment")
	}
}

func TestParseCompositePrimaryKey(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE post_tags (
			post_id INTEGER NOT NULL,
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

	// Composite PK columns should be NOT NULL.
	for _, colName := range pt.PrimaryKey.Columns {
		col := pt.FindColumn(colName)
		if col == nil {
			t.Fatalf("column %q not found", colName)
		}
		if col.IsNullable {
			t.Errorf("PK column %q should not be nullable", colName)
		}
	}
}

func TestParseInlineForeignKey(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE orgs (id INTEGER PRIMARY KEY);
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			org_id INTEGER NOT NULL REFERENCES orgs(id) ON DELETE CASCADE
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

func TestParseTableLevelForeignKey(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE posts (id INTEGER PRIMARY KEY);
		CREATE TABLE comments (
			id INTEGER PRIMARY KEY,
			post_id INTEGER NOT NULL,
			FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE ON UPDATE SET NULL
		);
	`)

	comments := findTable(s, "comments")
	if len(comments.ForeignKeys) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(comments.ForeignKeys))
	}

	fk := comments.ForeignKeys[0]
	if fk.RefTable != "posts" {
		t.Errorf("FK RefTable = %q, want posts", fk.RefTable)
	}
	if len(fk.Columns) != 1 || fk.Columns[0] != "post_id" {
		t.Errorf("FK Columns = %v, want [post_id]", fk.Columns)
	}
	if len(fk.RefColumns) != 1 || fk.RefColumns[0] != "id" {
		t.Errorf("FK RefColumns = %v, want [id]", fk.RefColumns)
	}
	if fk.OnDelete != schema.ActionCascade {
		t.Errorf("FK OnDelete = %q, want CASCADE", fk.OnDelete)
	}
	if fk.OnUpdate != schema.ActionSetNull {
		t.Errorf("FK OnUpdate = %q, want SET NULL", fk.OnUpdate)
	}
}

func TestParseNamedForeignKey(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE posts (id INTEGER PRIMARY KEY);
		CREATE TABLE comments (
			id INTEGER PRIMARY KEY,
			post_id INTEGER NOT NULL,
			CONSTRAINT fk_post FOREIGN KEY (post_id) REFERENCES posts(id)
		);
	`)

	comments := findTable(s, "comments")
	if len(comments.ForeignKeys) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(comments.ForeignKeys))
	}
	fk := comments.ForeignKeys[0]
	if fk.Name != "fk_post" {
		t.Errorf("FK name = %q, want fk_post", fk.Name)
	}
}

func TestParseCreateIndex(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT, org_id INTEGER);
		CREATE UNIQUE INDEX idx_email ON users (email);
		CREATE INDEX idx_org ON users (org_id);
	`)

	users := findTable(s, "users")
	if len(users.Indexes) != 2 {
		t.Fatalf("expected 2 indexes, got %d", len(users.Indexes))
	}

	idx0 := users.Indexes[0]
	if idx0.Name != "idx_email" || !idx0.Unique {
		t.Errorf("index 0: name=%q unique=%v, want idx_email/true", idx0.Name, idx0.Unique)
	}
	if len(idx0.Columns) != 1 || idx0.Columns[0] != "email" {
		t.Errorf("index 0 columns = %v, want [email]", idx0.Columns)
	}

	idx1 := users.Indexes[1]
	if idx1.Name != "idx_org" || idx1.Unique {
		t.Errorf("index 1: name=%q unique=%v, want idx_org/false", idx1.Name, idx1.Unique)
	}

	// Unique index should also create a UniqueConstraint.
	if len(users.Unique) != 1 {
		t.Fatalf("expected 1 unique constraint from index, got %d", len(users.Unique))
	}
}

func TestParseCreateView(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT, published BOOLEAN);
		CREATE VIEW published_posts AS SELECT id, title FROM posts WHERE published = 1;
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

func TestParseNotNullDefault(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE settings (
			key TEXT NOT NULL,
			value TEXT NOT NULL DEFAULT '',
			count INTEGER NOT NULL DEFAULT 0,
			active BOOLEAN NOT NULL DEFAULT 1
		);
	`)

	table := s.Tables[0]
	for _, col := range table.Columns {
		if col.IsNullable {
			t.Errorf("col %q: should be NOT NULL", col.Name)
		}
	}

	valCol := table.FindColumn("value")
	if !valCol.HasDefault {
		t.Error("value column should have default")
	}

	countCol := table.FindColumn("count")
	if !countCol.HasDefault {
		t.Error("count column should have default")
	}
}

func TestParseTypeAffinity(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE types_test (
			a INTEGER PRIMARY KEY,
			b VARCHAR(255) NOT NULL,
			c BOOLEAN,
			d DATETIME,
			e DOUBLE PRECISION,
			f NUMERIC,
			g BLOB,
			h REAL,
			i TEXT,
			j BIGINT
		);
	`)

	table := s.Tables[0]
	tests := []struct {
		col    string
		dbType string
	}{
		{"a", "integer"},
		{"b", "text"},
		{"c", "boolean"},
		{"d", "datetime"},
		{"e", "double precision"},
		{"f", "numeric"},
		{"g", "blob"},
		{"h", "double precision"},
		{"i", "text"},
		{"j", "bigint"},
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

func TestParseSelfReferencingFK(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE categories (
			id INTEGER PRIMARY KEY,
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

func TestParseMultipleFKs(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE users (id INTEGER PRIMARY KEY);
		CREATE TABLE messages (
			id INTEGER PRIMARY KEY,
			sender_id INTEGER NOT NULL REFERENCES users(id),
			recipient_id INTEGER NOT NULL REFERENCES users(id)
		);
	`)

	messages := findTable(s, "messages")
	if len(messages.ForeignKeys) != 2 {
		t.Fatalf("expected 2 FKs, got %d", len(messages.ForeignKeys))
	}

	if messages.ForeignKeys[0].Columns[0] != "sender_id" {
		t.Errorf("FK[0] column = %q, want sender_id", messages.ForeignKeys[0].Columns[0])
	}
	if messages.ForeignKeys[1].Columns[0] != "recipient_id" {
		t.Errorf("FK[1] column = %q, want recipient_id", messages.ForeignKeys[1].Columns[0])
	}
}

func TestParseMultipleStatements(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE a (id INTEGER PRIMARY KEY);
		CREATE TABLE b (id INTEGER PRIMARY KEY, a_id INTEGER REFERENCES a(id));
		CREATE VIEW v AS SELECT id FROM a;
	`)

	if len(s.Tables) != 2 {
		t.Errorf("tables = %d, want 2", len(s.Tables))
	}
	if len(s.Views) != 1 {
		t.Errorf("views = %d, want 1", len(s.Views))
	}
}

func TestParseComments(t *testing.T) {
	s := mustParse(t, `
		-- This is a comment
		CREATE TABLE users (
			id INTEGER PRIMARY KEY, -- inline comment
			name TEXT NOT NULL
		);
		/* Block comment */
		CREATE TABLE posts (id INTEGER PRIMARY KEY);
	`)

	if len(s.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(s.Tables))
	}
}

func TestParseIfNotExists(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL
		);
	`)

	if len(s.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(s.Tables))
	}
	if s.Tables[0].Name != "users" {
		t.Errorf("table name = %q, want users", s.Tables[0].Name)
	}
}

func TestParseCheckConstraintIgnored(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE products (
			id INTEGER PRIMARY KEY,
			price REAL NOT NULL CHECK (price > 0),
			name TEXT NOT NULL,
			CHECK (length(name) > 0)
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

func TestParseOnDeleteSetNull(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE users (id INTEGER PRIMARY KEY);
		CREATE TABLE audit_log (
			id INTEGER PRIMARY KEY,
			user_id INTEGER REFERENCES users(id) ON DELETE SET NULL
		);
	`)

	audit := findTable(s, "audit_log")
	if len(audit.ForeignKeys) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(audit.ForeignKeys))
	}
	if audit.ForeignKeys[0].OnDelete != schema.ActionSetNull {
		t.Errorf("FK OnDelete = %q, want SET NULL", audit.ForeignKeys[0].OnDelete)
	}
}

func TestParseFile(t *testing.T) {
	p := &Parser{}
	s, err := p.ParseFile("testdata/schema.sql")
	if err != nil {
		t.Fatalf("ParseFile() error: %v", err)
	}

	if len(s.Tables) != 6 {
		t.Errorf("expected 6 tables, got %d", len(s.Tables))
	}
	if len(s.Views) != 1 {
		t.Errorf("expected 1 view, got %d", len(s.Views))
	}

	// Check that indexes were applied.
	users := findTable(s, "users")
	if users == nil {
		t.Fatal("users table not found")
	}
	if len(users.Indexes) != 1 {
		t.Errorf("users indexes = %d, want 1", len(users.Indexes))
	}

	// Check named FK constraint.
	comments := findTable(s, "comments")
	if comments == nil {
		t.Fatal("comments table not found")
	}
	if len(comments.ForeignKeys) != 2 {
		t.Errorf("comments FKs = %d, want 2", len(comments.ForeignKeys))
	}
	for _, fk := range comments.ForeignKeys {
		if fk.Name == "" {
			t.Error("expected named FK constraint")
		}
	}

	// Check composite PK.
	postTags := findTable(s, "post_tags")
	if postTags == nil {
		t.Fatal("post_tags table not found")
	}
	if postTags.PrimaryKey == nil || len(postTags.PrimaryKey.Columns) != 2 {
		t.Error("post_tags should have composite PK with 2 columns")
	}
}

func TestParseQuotedIdentifiers(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE "My Table" (
			"Column One" INTEGER PRIMARY KEY,
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

func TestParseNoType(t *testing.T) {
	// SQLite allows columns with no type specification.
	s := mustParse(t, `
		CREATE TABLE dynamic (
			id INTEGER PRIMARY KEY,
			data
		);
	`)

	table := s.Tables[0]
	data := table.FindColumn("data")
	if data == nil {
		t.Fatal("data column not found")
	}
	// Column with no type should default to blob.
	if data.DBType != "blob" {
		t.Errorf("data.DBType = %q, want blob", data.DBType)
	}
}

func TestParseNamedUniqueConstraint(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			email TEXT NOT NULL,
			org_id INTEGER NOT NULL,
			CONSTRAINT uq_email_org UNIQUE (email, org_id)
		);
	`)

	users := s.Tables[0]
	if len(users.Unique) != 1 {
		t.Fatalf("expected 1 unique constraint, got %d", len(users.Unique))
	}
	uq := users.Unique[0]
	if uq.Name != "uq_email_org" {
		t.Errorf("unique constraint name = %q, want uq_email_org", uq.Name)
	}
	if len(uq.Columns) != 2 || uq.Columns[0] != "email" || uq.Columns[1] != "org_id" {
		t.Errorf("unique constraint columns = %v, want [email, org_id]", uq.Columns)
	}
}

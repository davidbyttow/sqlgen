package mysql

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
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			email VARCHAR(255) NOT NULL,
			name VARCHAR(255),
			age INT NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id)
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
		{"id", "bigint unsigned", false, true},
		{"email", "varchar", false, false},
		{"name", "varchar", true, false},
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
}

func TestParseAutoIncrement(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE items (
			id INT NOT NULL AUTO_INCREMENT,
			big_id BIGINT NOT NULL AUTO_INCREMENT,
			PRIMARY KEY (id)
		);
	`)

	items := s.Tables[0]
	tests := []struct {
		name    string
		autoInc bool
	}{
		{"id", true},
		{"big_id", true},
	}

	for i, tt := range tests {
		col := items.Columns[i]
		if col.IsAutoIncrement != tt.autoInc {
			t.Errorf("col %q: IsAutoIncrement = %v, want %v", col.Name, col.IsAutoIncrement, tt.autoInc)
		}
		if !col.HasDefault {
			t.Errorf("col %q: HasDefault should be true for AUTO_INCREMENT", col.Name)
		}
	}
}

func TestParseInlinePrimaryKey(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE items (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100) NOT NULL
		);
	`)

	items := s.Tables[0]
	if items.PrimaryKey == nil {
		t.Fatal("expected primary key")
	}
	if len(items.PrimaryKey.Columns) != 1 || items.PrimaryKey.Columns[0] != "id" {
		t.Errorf("PK columns = %v, want [id]", items.PrimaryKey.Columns)
	}

	id := items.FindColumn("id")
	if id.IsNullable {
		t.Error("PK column should not be nullable")
	}
}

func TestParseCompositePrimaryKey(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE post_tags (
			post_id BIGINT UNSIGNED NOT NULL,
			tag_id INT NOT NULL,
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

func TestParseTableLevelForeignKey(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE orgs (id INT PRIMARY KEY);
		CREATE TABLE users (
			id INT PRIMARY KEY,
			org_id INT NOT NULL,
			FOREIGN KEY (org_id) REFERENCES orgs(id) ON DELETE CASCADE
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

func TestParseInlineForeignKey(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE orgs (id INT PRIMARY KEY);
		CREATE TABLE users (
			id INT PRIMARY KEY,
			org_id INT NOT NULL REFERENCES orgs(id) ON DELETE SET NULL ON UPDATE CASCADE
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
	if fk.OnDelete != schema.ActionSetNull {
		t.Errorf("FK OnDelete = %q, want SET NULL", fk.OnDelete)
	}
	if fk.OnUpdate != schema.ActionCascade {
		t.Errorf("FK OnUpdate = %q, want CASCADE", fk.OnUpdate)
	}
}

func TestParseConstraintFK(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE orgs (id INT PRIMARY KEY);
		CREATE TABLE users (
			id INT PRIMARY KEY,
			org_id INT NOT NULL,
			CONSTRAINT fk_user_org FOREIGN KEY (org_id) REFERENCES orgs(id)
		);
	`)

	users := findTable(s, "users")
	if len(users.ForeignKeys) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(users.ForeignKeys))
	}
	fk := users.ForeignKeys[0]
	if fk.Name != "fk_user_org" {
		t.Errorf("FK name = %q, want fk_user_org", fk.Name)
	}
}

func TestParseEnum(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE users (
			id INT PRIMARY KEY,
			role ENUM('admin', 'member', 'guest') NOT NULL DEFAULT 'member'
		);
	`)

	if len(s.Enums) != 1 {
		t.Fatalf("expected 1 enum, got %d", len(s.Enums))
	}

	enum := s.Enums[0]
	if enum.Name != "users_role" {
		t.Errorf("enum name = %q, want users_role", enum.Name)
	}
	if len(enum.Values) != 3 {
		t.Fatalf("enum values = %v, want 3 values", enum.Values)
	}
	if enum.Values[0] != "admin" || enum.Values[1] != "member" || enum.Values[2] != "guest" {
		t.Errorf("enum values = %v", enum.Values)
	}

	// Column should reference the enum.
	users := s.Tables[0]
	role := users.FindColumn("role")
	if role == nil {
		t.Fatal("role column not found")
	}
	if role.EnumName != "users_role" {
		t.Errorf("role.EnumName = %q, want users_role", role.EnumName)
	}
	if role.DBType != "enum" {
		t.Errorf("role.DBType = %q, want enum", role.DBType)
	}
	if role.HasDefault != true {
		t.Error("role should have default")
	}
}

func TestParseView(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE posts (id INT PRIMARY KEY, title VARCHAR(255), published TINYINT(1));
		CREATE VIEW published_posts AS SELECT id, title FROM posts WHERE published = 1;
	`)

	if len(s.Views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(s.Views))
	}
	v := s.Views[0]
	if v.Name != "published_posts" {
		t.Errorf("view name = %q, want published_posts", v.Name)
	}
	if v.Query == "" {
		t.Error("view query should not be empty")
	}
}

func TestParseBacktickQuoting(t *testing.T) {
	s := mustParse(t, "CREATE TABLE `my_table` (\n`id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,\n`select` VARCHAR(100) NOT NULL,\n`name` TEXT\n);")

	table := findTable(s, "my_table")
	if table == nil {
		t.Fatal("table my_table not found")
	}
	if len(table.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(table.Columns))
	}
	if table.Columns[0].Name != "id" {
		t.Errorf("col[0].Name = %q, want id", table.Columns[0].Name)
	}
	if table.Columns[1].Name != "select" {
		t.Errorf("col[1].Name = %q, want select", table.Columns[1].Name)
	}
}

func TestParseUnsigned(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE nums (
			id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
			tiny TINYINT UNSIGNED,
			small SMALLINT UNSIGNED,
			med MEDIUMINT UNSIGNED,
			big BIGINT UNSIGNED
		);
	`)

	table := s.Tables[0]
	tests := []struct {
		name   string
		dbType string
	}{
		{"id", "integer unsigned"},
		{"tiny", "tinyint unsigned"},
		{"small", "smallint unsigned"},
		{"med", "mediumint unsigned"},
		{"big", "bigint unsigned"},
	}

	for _, tt := range tests {
		col := table.FindColumn(tt.name)
		if col == nil {
			t.Errorf("column %q not found", tt.name)
			continue
		}
		if col.DBType != tt.dbType {
			t.Errorf("col %q: DBType = %q, want %q", tt.name, col.DBType, tt.dbType)
		}
	}
}

func TestParseMySQLTypes(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE types_test (
			a INT PRIMARY KEY,
			b VARCHAR(255) NOT NULL,
			c BOOLEAN,
			d DATETIME,
			e DOUBLE,
			f DECIMAL(10, 2),
			g JSON,
			h BLOB,
			i TEXT,
			j TIMESTAMP,
			k TINYINT(1),
			l BIGINT UNSIGNED,
			m FLOAT,
			n DATE,
			o TIME,
			p YEAR,
			q MEDIUMTEXT,
			r LONGTEXT,
			s BINARY(16),
			u CHAR(10)
		);
	`)

	table := s.Tables[0]
	tests := []struct {
		col    string
		dbType string
	}{
		{"a", "integer"},
		{"b", "varchar"},
		{"c", "boolean"},
		{"d", "datetime"},
		{"e", "double"},
		{"f", "decimal"},
		{"g", "json"},
		{"h", "blob"},
		{"i", "text"},
		{"j", "timestamp"},
		{"k", "boolean"},
		{"l", "bigint unsigned"},
		{"m", "float"},
		{"n", "date"},
		{"o", "time"},
		{"p", "year"},
		{"q", "mediumtext"},
		{"r", "longtext"},
		{"s", "binary"},
		{"u", "char"},
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

func TestParseCreateIndex(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE users (id INT PRIMARY KEY, email VARCHAR(255), org_id INT);
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

func TestParseTableOptions(t *testing.T) {
	// Table options (ENGINE, CHARSET, etc.) should be parsed but ignored.
	s := mustParse(t, `
		CREATE TABLE items (
			id INT NOT NULL AUTO_INCREMENT,
			name VARCHAR(255) NOT NULL,
			PRIMARY KEY (id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`)

	if len(s.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(s.Tables))
	}
	if s.Tables[0].Name != "items" {
		t.Errorf("table name = %q, want items", s.Tables[0].Name)
	}
}

func TestParseAlterTableAddConstraint(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE orgs (id INT PRIMARY KEY);
		CREATE TABLE users (id INT PRIMARY KEY, org_id INT NOT NULL);
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

func TestParseSelfReferencingFK(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE categories (
			id INT NOT NULL AUTO_INCREMENT,
			name VARCHAR(255) NOT NULL,
			parent_id INT,
			PRIMARY KEY (id),
			FOREIGN KEY (parent_id) REFERENCES categories(id)
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

func TestParseForeignKeyActions(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE parent (id INT PRIMARY KEY);
		CREATE TABLE child (
			id INT PRIMARY KEY,
			parent_id INT,
			FOREIGN KEY (parent_id) REFERENCES parent(id) ON DELETE CASCADE ON UPDATE SET NULL
		);
	`)

	child := findTable(s, "child")
	if len(child.ForeignKeys) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(child.ForeignKeys))
	}

	fk := child.ForeignKeys[0]
	if fk.OnDelete != schema.ActionCascade {
		t.Errorf("FK OnDelete = %q, want CASCADE", fk.OnDelete)
	}
	if fk.OnUpdate != schema.ActionSetNull {
		t.Errorf("FK OnUpdate = %q, want SET NULL", fk.OnUpdate)
	}
}

func TestParseMultipleStatements(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE a (id INT AUTO_INCREMENT PRIMARY KEY);
		CREATE TABLE b (id INT AUTO_INCREMENT PRIMARY KEY, a_id INT, FOREIGN KEY (a_id) REFERENCES a(id));
		CREATE VIEW v AS SELECT id FROM a;
	`)

	if len(s.Tables) != 2 {
		t.Errorf("tables = %d, want 2", len(s.Tables))
	}
	if len(s.Views) != 1 {
		t.Errorf("views = %d, want 1", len(s.Views))
	}
}

func TestParseUniqueConstraint(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE users (
			id INT PRIMARY KEY,
			email VARCHAR(255) NOT NULL,
			UNIQUE KEY uk_email (email)
		);
	`)

	users := s.Tables[0]
	if len(users.Unique) != 1 {
		t.Fatalf("expected 1 unique constraint, got %d", len(users.Unique))
	}
	if users.Unique[0].Name != "uk_email" {
		t.Errorf("unique name = %q, want uk_email", users.Unique[0].Name)
	}
	if users.Unique[0].Columns[0] != "email" {
		t.Errorf("unique columns = %v, want [email]", users.Unique[0].Columns)
	}
}

func TestParseInlineUnique(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE tags (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100) NOT NULL UNIQUE
		);
	`)

	tags := s.Tables[0]
	if len(tags.Unique) != 1 {
		t.Fatalf("expected 1 unique constraint, got %d", len(tags.Unique))
	}
	if tags.Unique[0].Columns[0] != "name" {
		t.Errorf("unique columns = %v, want [name]", tags.Unique[0].Columns)
	}
}

func TestParseNotNull(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE test (
			a INT NOT NULL,
			b VARCHAR(255),
			c TEXT NOT NULL,
			d DATETIME
		);
	`)

	table := s.Tables[0]
	tests := []struct {
		name     string
		nullable bool
	}{
		{"a", false},
		{"b", true},
		{"c", false},
		{"d", true},
	}

	for _, tt := range tests {
		col := table.FindColumn(tt.name)
		if col.IsNullable != tt.nullable {
			t.Errorf("col %q: IsNullable = %v, want %v", tt.name, col.IsNullable, tt.nullable)
		}
	}
}

func TestParseTinyintBoolean(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE flags (
			id INT PRIMARY KEY,
			active TINYINT(1) NOT NULL DEFAULT 1,
			counter TINYINT NOT NULL DEFAULT 0
		);
	`)

	table := s.Tables[0]
	active := table.FindColumn("active")
	if active.DBType != "boolean" {
		t.Errorf("active DBType = %q, want boolean", active.DBType)
	}

	counter := table.FindColumn("counter")
	if counter.DBType != "tinyint" {
		t.Errorf("counter DBType = %q, want tinyint", counter.DBType)
	}
}

func TestParseCompositeFK(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE parent (
			a INT,
			b INT,
			PRIMARY KEY (a, b)
		);
		CREATE TABLE child (
			id INT PRIMARY KEY,
			parent_a INT NOT NULL,
			parent_b INT NOT NULL,
			FOREIGN KEY (parent_a, parent_b) REFERENCES parent(a, b)
		);
	`)

	child := findTable(s, "child")
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

func TestParseInlineIndex(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE users (
			id INT PRIMARY KEY,
			email VARCHAR(255),
			org_id INT,
			INDEX idx_org (org_id),
			KEY idx_email (email)
		);
	`)

	users := s.Tables[0]
	if len(users.Indexes) != 2 {
		t.Fatalf("expected 2 indexes, got %d", len(users.Indexes))
	}

	idx0 := users.Indexes[0]
	if idx0.Name != "idx_org" {
		t.Errorf("index 0 name = %q, want idx_org", idx0.Name)
	}
	if idx0.Unique {
		t.Error("index 0 should not be unique")
	}

	idx1 := users.Indexes[1]
	if idx1.Name != "idx_email" {
		t.Errorf("index 1 name = %q, want idx_email", idx1.Name)
	}
}

func TestParseIfNotExists(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE IF NOT EXISTS items (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100) NOT NULL
		);
	`)

	if len(s.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(s.Tables))
	}
	if s.Tables[0].Name != "items" {
		t.Errorf("table name = %q, want items", s.Tables[0].Name)
	}
}

func TestParseComments(t *testing.T) {
	s := mustParse(t, `
		-- This is a comment
		CREATE TABLE items (
			id INT AUTO_INCREMENT PRIMARY KEY, -- inline comment
			name VARCHAR(100) NOT NULL
		);
		# Another comment style
		/* Block comment */
	`)

	if len(s.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(s.Tables))
	}
}

func TestParseFile(t *testing.T) {
	p := &Parser{}
	s, err := p.ParseFile("testdata/schema.sql")
	if err != nil {
		t.Fatalf("ParseFile() error: %v", err)
	}

	// Check counts from our test fixture.
	if len(s.Tables) < 7 {
		t.Errorf("expected at least 7 tables, got %d", len(s.Tables))
	}
	if len(s.Enums) < 1 {
		t.Errorf("expected at least 1 enum, got %d", len(s.Enums))
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
	hasIdx := false
	for _, idx := range users.Indexes {
		if idx.Name == "idx_users_email_org" && idx.Unique {
			hasIdx = true
			break
		}
	}
	if !hasIdx {
		t.Error("expected unique index idx_users_email_org on users")
	}
}

func TestParseDefaultExpressions(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE defaults_test (
			id INT AUTO_INCREMENT PRIMARY KEY,
			counter INT NOT NULL DEFAULT 0,
			label VARCHAR(100) NOT NULL DEFAULT 'unnamed',
			active BOOLEAN NOT NULL DEFAULT true,
			data JSON NOT NULL DEFAULT ('{}'),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`)

	table := findTable(s, "defaults_test")
	if table == nil {
		t.Fatal("defaults_test table not found")
	}

	for _, col := range table.Columns {
		if !col.HasDefault {
			t.Errorf("col %q: HasDefault = false, want true", col.Name)
		}
	}
}

func TestParseOrReplaceView(t *testing.T) {
	s := mustParse(t, `
		CREATE TABLE posts (id INT PRIMARY KEY, title VARCHAR(255));
		CREATE OR REPLACE VIEW recent_posts AS SELECT id, title FROM posts;
	`)

	if len(s.Views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(s.Views))
	}
	if s.Views[0].Name != "recent_posts" {
		t.Errorf("view name = %q, want recent_posts", s.Views[0].Name)
	}
}

func TestParseCharacterSetAndCollate(t *testing.T) {
	// CHARACTER SET and COLLATE on columns should be parsed but ignored.
	s := mustParse(t, `
		CREATE TABLE texts (
			id INT PRIMARY KEY,
			content VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
			summary TEXT CHARSET utf8 NOT NULL
		);
	`)

	table := s.Tables[0]
	content := table.FindColumn("content")
	if content == nil {
		t.Fatal("content column not found")
	}
	if content.DBType != "varchar" {
		t.Errorf("content.DBType = %q, want varchar", content.DBType)
	}
	if content.IsNullable {
		t.Error("content should not be nullable")
	}
}

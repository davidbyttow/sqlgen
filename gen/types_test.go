package gen

import (
	"testing"

	"github.com/davidbyttow/sqlgen/config"
	"github.com/davidbyttow/sqlgen/schema"
)

func newMapper(nullType config.NullType, replacements map[string]string) *TypeMapper {
	cfg := &config.Config{
		Input:  config.InputConfig{Dialect: "postgres", Paths: []string{"x"}},
		Output: config.OutputConfig{Dir: "out"},
		Types:  config.TypesConfig{NullType: nullType, Replacements: replacements},
	}
	return NewTypeMapper(cfg, "github.com/davidbyttow/sqlgen")
}

func TestGoTypeForBasic(t *testing.T) {
	m := newMapper(config.NullTypeGeneric, nil)

	tests := []struct {
		col  schema.Column
		want string
	}{
		{schema.Column{DBType: "text"}, "string"},
		{schema.Column{DBType: "integer"}, "int32"},
		{schema.Column{DBType: "bigint"}, "int64"},
		{schema.Column{DBType: "boolean"}, "bool"},
		{schema.Column{DBType: "timestamp with time zone"}, "time.Time"},
		{schema.Column{DBType: "uuid"}, "string"},
		{schema.Column{DBType: "jsonb"}, "json.RawMessage"},
		{schema.Column{DBType: "bytea"}, "[]byte"},
		{schema.Column{DBType: "double precision"}, "float64"},
	}

	for _, tt := range tests {
		t.Run(tt.col.DBType, func(t *testing.T) {
			got := m.GoTypeFor(&tt.col)
			if got.Name != tt.want {
				t.Errorf("GoTypeFor(%q) = %q, want %q", tt.col.DBType, got.Name, tt.want)
			}
		})
	}
}

func TestGoTypeForNullableGeneric(t *testing.T) {
	m := newMapper(config.NullTypeGeneric, nil)

	col := &schema.Column{DBType: "text", IsNullable: true}
	got := m.GoTypeFor(col)
	if got.Name != "sqlgen.Null[string]" {
		t.Errorf("nullable text = %q, want sqlgen.Null[string]", got.Name)
	}
	if got.Import != "github.com/davidbyttow/sqlgen" {
		t.Errorf("import = %q", got.Import)
	}
}

func TestGoTypeForNullablePointer(t *testing.T) {
	m := newMapper(config.NullTypePointer, nil)

	col := &schema.Column{DBType: "text", IsNullable: true}
	got := m.GoTypeFor(col)
	if got.Name != "*string" {
		t.Errorf("nullable text = %q, want *string", got.Name)
	}
}

func TestGoTypeForNullableDatabase(t *testing.T) {
	m := newMapper(config.NullTypeDatabase, nil)

	tests := []struct {
		dbType string
		want   string
	}{
		{"text", "sql.NullString"},
		{"integer", "sql.NullInt32"},
		{"bigint", "sql.NullInt64"},
		{"boolean", "sql.NullBool"},
		{"timestamp with time zone", "sql.NullTime"},
		{"double precision", "sql.NullFloat64"},
	}

	for _, tt := range tests {
		col := &schema.Column{DBType: tt.dbType, IsNullable: true}
		got := m.GoTypeFor(col)
		if got.Name != tt.want {
			t.Errorf("nullable %q = %q, want %q", tt.dbType, got.Name, tt.want)
		}
	}
}

func TestGoTypeForArray(t *testing.T) {
	m := newMapper(config.NullTypeGeneric, nil)

	col := &schema.Column{DBType: "text", IsArray: true, ArrayDims: 1}
	got := m.GoTypeFor(col)
	if got.Name != "[]string" {
		t.Errorf("text[] = %q, want []string", got.Name)
	}
}

func TestGoTypeForEnum(t *testing.T) {
	m := newMapper(config.NullTypeGeneric, nil)

	col := &schema.Column{DBType: "user_role", EnumName: "user_role"}
	got := m.GoTypeFor(col)
	if got.Name != "UserRole" {
		t.Errorf("enum = %q, want UserRole", got.Name)
	}

	col.IsNullable = true
	got = m.GoTypeFor(col)
	if got.Name != "sqlgen.Null[UserRole]" {
		t.Errorf("nullable enum = %q, want sqlgen.Null[UserRole]", got.Name)
	}
}

func TestGoTypeForReplacement(t *testing.T) {
	m := newMapper(config.NullTypeGeneric, map[string]string{
		"uuid": "github.com/google/uuid.UUID",
	})

	col := &schema.Column{DBType: "uuid"}
	got := m.GoTypeFor(col)
	if got.Name != "uuid.UUID" {
		t.Errorf("uuid replacement = %q, want uuid.UUID", got.Name)
	}
	if got.Import != "github.com/google/uuid" {
		t.Errorf("uuid import = %q", got.Import)
	}
}

func TestGoTypeForTableColumnReplacement(t *testing.T) {
	cfg := &config.Config{
		Input:  config.InputConfig{Dialect: "postgres", Paths: []string{"x"}},
		Output: config.OutputConfig{Dir: "out"},
		Types: config.TypesConfig{
			NullType: config.NullTypeGeneric,
			ColumnReplacements: map[string]string{
				"users.metadata": "map[string]any",
				"*.external_id":  "github.com/google/uuid.UUID",
			},
		},
	}
	m := NewTypeMapper(cfg, "github.com/davidbyttow/sqlgen")

	// Exact match: users.metadata -> map[string]any
	col := &schema.Column{Name: "metadata", DBType: "jsonb"}
	got := m.GoTypeForTable(col, "users")
	if got.Name != "map[string]any" {
		t.Errorf("users.metadata = %q, want map[string]any", got.Name)
	}

	// Same column on different table: falls through to default jsonb mapping.
	got = m.GoTypeForTable(col, "posts")
	if got.Name != "json.RawMessage" {
		t.Errorf("posts.metadata = %q, want json.RawMessage", got.Name)
	}

	// Wildcard match: *.external_id -> uuid.UUID
	col2 := &schema.Column{Name: "external_id", DBType: "uuid"}
	got = m.GoTypeForTable(col2, "orders")
	if got.Name != "uuid.UUID" {
		t.Errorf("orders.external_id = %q, want uuid.UUID", got.Name)
	}
	if got.Import != "github.com/google/uuid" {
		t.Errorf("import = %q, want github.com/google/uuid", got.Import)
	}

	// No table name: column replacements are skipped.
	got = m.GoTypeFor(col)
	if got.Name != "json.RawMessage" {
		t.Errorf("GoTypeFor(metadata) = %q, want json.RawMessage", got.Name)
	}
}

func TestParseTypeString(t *testing.T) {
	tests := []struct {
		input      string
		wantName   string
		wantImport string
	}{
		{"string", "string", ""},
		{"int64", "int64", ""},
		{"github.com/google/uuid.UUID", "uuid.UUID", "github.com/google/uuid"},
		{"github.com/shopspring/decimal.Decimal", "decimal.Decimal", "github.com/shopspring/decimal"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseTypeString(tt.input)
			if got.Name != tt.wantName {
				t.Errorf("name = %q, want %q", got.Name, tt.wantName)
			}
			if got.Import != tt.wantImport {
				t.Errorf("import = %q, want %q", got.Import, tt.wantImport)
			}
		})
	}
}

package config

import "testing"

func TestParse(t *testing.T) {
	yaml := `
input:
  dialect: postgres
  paths:
    - schema.sql
    - migrations/
output:
  dir: models
  package: models
types:
  "null": generic
  replacements:
    uuid: "github.com/google/uuid.UUID"
tables:
  users:
    columns:
      email:
        name: EmailAddress
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if cfg.Input.Dialect != "postgres" {
		t.Errorf("dialect = %q, want postgres", cfg.Input.Dialect)
	}
	if len(cfg.Input.Paths) != 2 {
		t.Errorf("paths len = %d, want 2", len(cfg.Input.Paths))
	}
	if cfg.Output.Dir != "models" {
		t.Errorf("output.dir = %q, want models", cfg.Output.Dir)
	}
	if cfg.Types.NullType != NullTypeGeneric {
		t.Errorf("null type = %q, want generic", cfg.Types.NullType)
	}
	if cfg.Types.Replacements["uuid"] != "github.com/google/uuid.UUID" {
		t.Errorf("uuid replacement = %q", cfg.Types.Replacements["uuid"])
	}
	tc, ok := cfg.Tables["users"]
	if !ok {
		t.Fatal("missing users table config")
	}
	if tc.Columns["email"].Name != "EmailAddress" {
		t.Errorf("email column name = %q, want EmailAddress", tc.Columns["email"].Name)
	}
}

func TestParseDefaults(t *testing.T) {
	yaml := `
input:
  paths: [schema.sql]
output:
  dir: out
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if cfg.Input.Dialect != "postgres" {
		t.Errorf("default dialect = %q, want postgres", cfg.Input.Dialect)
	}
	if cfg.Types.NullType != NullTypeGeneric {
		t.Errorf("default null type = %q, want generic", cfg.Types.NullType)
	}
}

func TestParseDSN(t *testing.T) {
	yaml := `
input:
  dialect: postgres
  dsn: "postgres://localhost:5432/mydb"
output:
  dir: models
  package: models
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if cfg.Input.DSN != "postgres://localhost:5432/mydb" {
		t.Errorf("dsn = %q, want postgres://localhost:5432/mydb", cfg.Input.DSN)
	}
	if len(cfg.Input.Paths) != 0 {
		t.Errorf("paths should be empty when DSN is set, got %v", cfg.Input.Paths)
	}
}

func TestParseDSNEnvExpansion(t *testing.T) {
	t.Setenv("TEST_SQLGEN_DSN", "postgres://expanded:5432/db")
	yaml := `
input:
  dsn: "${TEST_SQLGEN_DSN}"
output:
  dir: out
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if cfg.Input.DSN != "postgres://expanded:5432/db" {
		t.Errorf("dsn = %q, want expanded value", cfg.Input.DSN)
	}
}

func TestParseValidation(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{"no paths or dsn", `
input:
  dialect: postgres
output:
  dir: out
`},
		{"paths and dsn together", `
input:
  dialect: postgres
  paths: [schema.sql]
  dsn: "postgres://localhost/db"
output:
  dir: out
`},
		{"no output dir", `
input:
  paths: [schema.sql]
output:
  package: models
`},
		{"bad dialect", `
input:
  dialect: oracle
  paths: [schema.sql]
output:
  dir: out
`},
		{"bad null type", `
input:
  paths: [schema.sql]
output:
  dir: out
types:
  "null": custom
`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}

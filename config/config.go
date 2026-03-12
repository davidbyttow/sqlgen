// Package config handles sqlgen configuration parsing and validation.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level sqlgen configuration.
type Config struct {
	// Input specifies where to read the schema from.
	Input InputConfig `yaml:"input"`

	// Output specifies where and how to write generated code.
	Output OutputConfig `yaml:"output"`

	// Types configures type mapping behavior.
	Types TypesConfig `yaml:"types"`

	// Timestamps configures automatic timestamp management.
	Timestamps TimestampsConfig `yaml:"timestamps"`

	// Tags configures struct tag generation. Keys are tag names, values are casing.
	// Options: "snake", "camel", "pascal", "none" (raw DB column name).
	// Default: {"json": "snake", "db": "none"}
	Tags map[string]string `yaml:"tags"`

	// Tables allows per-table configuration overrides.
	Tables map[string]TableConfig `yaml:"tables"`

	// Polymorphic defines polymorphic relationships that can't be detected from DDL.
	// Each entry describes a type+id column pair and the tables it references.
	Polymorphic []PolymorphicConfig `yaml:"polymorphic"`
}

// PolymorphicConfig defines a polymorphic relationship.
type PolymorphicConfig struct {
	// Table is the table containing the type+id columns.
	Table string `yaml:"table"`

	// TypeColumn is the column holding the type discriminator (e.g., "commentable_type").
	TypeColumn string `yaml:"type_column"`

	// IDColumn is the column holding the FK value (e.g., "commentable_id").
	IDColumn string `yaml:"id_column"`

	// Targets maps type discriminator values to target tables.
	// Example: {"User": "users", "Post": "posts"}
	Targets map[string]string `yaml:"targets"`
}

// InputConfig specifies schema input sources.
type InputConfig struct {
	// Dialect is the SQL dialect to parse. Currently only "postgres".
	Dialect string `yaml:"dialect"`

	// Paths is a list of SQL files or directories to parse.
	// Mutually exclusive with DSN.
	Paths []string `yaml:"paths"`

	// DSN is a database connection string for live introspection.
	// Supports environment variable expansion (e.g., ${DATABASE_URL}).
	// Mutually exclusive with Paths.
	DSN string `yaml:"dsn"`
}

// OutputConfig specifies code generation output.
type OutputConfig struct {
	// Dir is the directory to write generated files to.
	Dir string `yaml:"dir"`

	// Package is the Go package name for generated code. Defaults to the dir basename.
	Package string `yaml:"package"`

	// Tests enables generation of _test.go files alongside models.
	Tests bool `yaml:"tests"`

	// NoHooks disables generation of hook files and hook calls in CRUD.
	NoHooks bool `yaml:"no_hooks"`

	// Templates is an optional path to a directory of .tmpl files.
	// Files matching built-in template names override them.
	// Extra .tmpl files are rendered once per table.
	Templates string `yaml:"templates"`

	// Factories enables generation of factory functions for testing.
	Factories bool `yaml:"factories"`
}

// TimestampsConfig controls automatic timestamp management.
type TimestampsConfig struct {
	// CreatedAt is the column name for creation timestamp. Default: "created_at".
	// Set to "-" to disable.
	CreatedAt string `yaml:"created_at"`

	// UpdatedAt is the column name for update timestamp. Default: "updated_at".
	// Set to "-" to disable.
	UpdatedAt string `yaml:"updated_at"`
}

// NullType determines how nullable columns are represented in Go.
type NullType string

const (
	NullTypeGeneric  NullType = "generic"  // Null[T] (default, built-in generic type)
	NullTypePointer  NullType = "pointer"  // *T
	NullTypeDatabase NullType = "database" // sql.NullString, sql.NullInt64, etc.
)

// TypesConfig configures type mapping behavior.
type TypesConfig struct {
	// NullType determines how nullable columns are represented.
	// Options: "generic" (default), "pointer", "database"
	NullType NullType `yaml:"null"`

	// Replacements maps DB type patterns to custom Go types.
	// Example: {"uuid": "github.com/google/uuid.UUID"}
	Replacements map[string]string `yaml:"replacements"`

	// ColumnReplacements maps "table.column" patterns to custom Go types.
	// Use "*" as table name to match all tables.
	// Examples:
	//   "users.metadata": "encoding/json.RawMessage"
	//   "*.external_id": "github.com/google/uuid.UUID"
	ColumnReplacements map[string]string `yaml:"column_replacements"`
}

// TableConfig provides per-table overrides.
type TableConfig struct {
	// Skip excludes this table from code generation.
	Skip bool `yaml:"skip"`

	// Name overrides the Go struct name for this table.
	Name string `yaml:"name"`

	// Columns provides per-column overrides.
	Columns map[string]ColumnConfig `yaml:"columns"`
}

// ColumnConfig provides per-column overrides.
type ColumnConfig struct {
	// Name overrides the Go field name for this column.
	Name string `yaml:"name"`

	// Type overrides the Go type for this column.
	Type string `yaml:"type"`
}

// Load reads and parses a config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	return Parse(data)
}

// Parse parses config from YAML bytes.
func Parse(data []byte) (*Config, error) {
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.Input.Dialect == "" {
		c.Input.Dialect = "postgres"
	}
	switch c.Input.Dialect {
	case "postgres", "sqlite", "mysql":
		// valid
	default:
		return fmt.Errorf("unsupported dialect: %q (supported: \"postgres\", \"sqlite\", \"mysql\")", c.Input.Dialect)
	}
	// Expand env vars in DSN.
	if c.Input.DSN != "" {
		c.Input.DSN = os.ExpandEnv(c.Input.DSN)
	}
	if len(c.Input.Paths) == 0 && c.Input.DSN == "" {
		return fmt.Errorf("input requires either paths or dsn")
	}
	if len(c.Input.Paths) > 0 && c.Input.DSN != "" {
		return fmt.Errorf("input.paths and input.dsn are mutually exclusive")
	}
	if c.Output.Dir == "" {
		return fmt.Errorf("output.dir is required: specify the output directory for generated code")
	}
	if c.Output.Templates != "" {
		if info, err := os.Stat(c.Output.Templates); err != nil {
			return fmt.Errorf("output.templates: %w", err)
		} else if !info.IsDir() {
			return fmt.Errorf("output.templates: %q is not a directory", c.Output.Templates)
		}
	}
	if c.Types.NullType == "" {
		c.Types.NullType = NullTypeGeneric
	}
	if c.Timestamps.CreatedAt == "" {
		c.Timestamps.CreatedAt = "created_at"
	}
	if c.Timestamps.UpdatedAt == "" {
		c.Timestamps.UpdatedAt = "updated_at"
	}
	if len(c.Tags) == 0 {
		c.Tags = map[string]string{"json": "snake", "db": "none"}
	}
	for tag, casing := range c.Tags {
		switch casing {
		case "snake", "camel", "pascal", "none":
			// valid
		default:
			return fmt.Errorf("unsupported casing %q for tag %q (options: \"snake\", \"camel\", \"pascal\", \"none\")", casing, tag)
		}
	}
	switch c.Types.NullType {
	case NullTypeGeneric, NullTypePointer, NullTypeDatabase:
		// valid
	default:
		return fmt.Errorf("unsupported types.null: %q (options: \"generic\", \"pointer\", \"database\")", c.Types.NullType)
	}
	return nil
}

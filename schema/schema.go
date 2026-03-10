// Package schema defines the intermediate representation (IR) for database schemas.
// All parsers (Postgres DDL, MySQL DDL, live DB introspection) produce this IR,
// and all code generators consume it.
package schema

// Schema is the top-level container for a parsed database schema.
type Schema struct {
	// Name is the schema name (e.g., "public" for Postgres default).
	Name string

	Tables []*Table
	Enums  []*Enum
	Views  []*View
}

// FindTable returns the table with the given name, or nil.
func (s *Schema) FindTable(name string) *Table {
	for _, t := range s.Tables {
		if t.Name == name {
			return t
		}
	}
	return nil
}

// Table represents a database table.
type Table struct {
	Schema      string // Schema name (e.g., "public")
	Name        string // Table name as it appears in the DB
	Columns     []*Column
	PrimaryKey  *PrimaryKey
	ForeignKeys []*ForeignKey
	Indexes     []*Index
	Unique      []*UniqueConstraint

	// Relationships are inferred from foreign keys after parsing.
	Relationships []*Relationship
}

// FullName returns the schema-qualified table name.
func (t *Table) FullName() string {
	if t.Schema == "" || t.Schema == "public" {
		return t.Name
	}
	return t.Schema + "." + t.Name
}

// FindColumn returns the column with the given name, or nil.
func (t *Table) FindColumn(name string) *Column {
	for _, c := range t.Columns {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// Column represents a single column in a table.
type Column struct {
	Name       string
	DBType     string // Raw DB type string (e.g., "uuid", "text", "integer")
	IsNullable bool
	HasDefault bool
	DefaultVal string // Raw default expression as string
	IsArray    bool
	ArrayDims  int // Number of array dimensions (1 for int[], 2 for int[][], etc.)

	// IsAutoIncrement is true for SERIAL/BIGSERIAL or GENERATED ALWAYS AS IDENTITY.
	IsAutoIncrement bool

	// EnumName is set when the column type is a user-defined enum.
	EnumName string
}

// PrimaryKey represents a table's primary key constraint.
type PrimaryKey struct {
	Name    string   // Constraint name (may be empty)
	Columns []string // Column names in PK order
}

// ForeignKey represents a foreign key constraint.
type ForeignKey struct {
	Name       string   // Constraint name
	Columns    []string // Local column names
	RefTable   string   // Referenced table name
	RefSchema  string   // Referenced table schema
	RefColumns []string // Referenced column names
	OnDelete   Action
	OnUpdate   Action
}

// Action represents a foreign key referential action.
type Action string

const (
	ActionNone       Action = ""
	ActionCascade    Action = "CASCADE"
	ActionSetNull    Action = "SET NULL"
	ActionSetDefault Action = "SET DEFAULT"
	ActionRestrict   Action = "RESTRICT"
	ActionNoAction   Action = "NO ACTION"
)

// Index represents a database index.
type Index struct {
	Name    string
	Columns []string
	Unique  bool
}

// UniqueConstraint represents a UNIQUE constraint on one or more columns.
type UniqueConstraint struct {
	Name    string
	Columns []string
}

// Enum represents a database enum type.
type Enum struct {
	Schema string
	Name   string
	Values []string
}

// View represents a database view or materialized view.
type View struct {
	Schema         string
	Name           string
	Columns        []*Column
	IsMaterialized bool
	// Query is the raw SQL of the view definition, if available.
	Query string
}

// RelationType classifies the kind of relationship between two tables.
type RelationType int

const (
	RelBelongsTo  RelationType = iota // This table has the FK (many-to-one)
	RelHasOne                         // Other table has FK pointing here, unique constraint
	RelHasMany                        // Other table has FK pointing here
	RelManyToMany                     // Join table connects two tables
)

// Relationship represents an inferred relationship between tables.
type Relationship struct {
	Type RelationType

	// Local side
	Table   string   // Table this relationship is defined on
	Columns []string // Local columns involved

	// Foreign side
	ForeignTable   string   // The other table
	ForeignColumns []string // Columns on the other table

	// For many-to-many: the join table details
	JoinTable          string
	JoinLocalColumns   []string // Join table columns pointing to local table
	JoinForeignColumns []string // Join table columns pointing to foreign table

	// ForeignKey is the FK that produced this relationship.
	ForeignKey *ForeignKey
}

// Parser is the interface that all DDL parsers implement.
type Parser interface {
	// ParseFile parses a single SQL file and returns the schema objects found.
	ParseFile(path string) (*Schema, error)

	// ParseDir parses all SQL files in a directory and merges them into one Schema.
	ParseDir(dir string) (*Schema, error)

	// ParseString parses SQL from a string.
	ParseString(sql string) (*Schema, error)
}

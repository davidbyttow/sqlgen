// Package gen implements the code generation engine for sqlgen.
package gen

import (
	"fmt"

	"github.com/davidbyttow/sqlgen/config"
	"github.com/davidbyttow/sqlgen/internal/naming"
	"github.com/davidbyttow/sqlgen/schema"
)

// GoType represents a Go type with its import path.
type GoType struct {
	Name   string // e.g., "string", "int64", "uuid.UUID", "Null[string]"
	Import string // e.g., "", "time", "github.com/google/uuid"
}

// TypeMapper converts database column types to Go types.
type TypeMapper struct {
	nullType           config.NullType
	replacements       map[string]string
	columnReplacements map[string]string
	runtimePkg         string // Import path for the runtime package
}

// NewTypeMapper creates a TypeMapper from config.
func NewTypeMapper(cfg *config.Config, runtimePkg string) *TypeMapper {
	return &TypeMapper{
		nullType:           cfg.Types.NullType,
		replacements:       cfg.Types.Replacements,
		columnReplacements: cfg.Types.ColumnReplacements,
		runtimePkg:         runtimePkg,
	}
}

// GoTypeFor returns the Go type for a schema column.
func (m *TypeMapper) GoTypeFor(col *schema.Column) GoType {
	return m.GoTypeForTable(col, "")
}

// GoTypeForTable returns the Go type for a column, considering table-specific overrides.
func (m *TypeMapper) GoTypeForTable(col *schema.Column, tableName string) GoType {
	// Check column-level replacements first (most specific).
	if m.columnReplacements != nil && tableName != "" {
		// Exact match: "table.column"
		if repl, ok := m.columnReplacements[tableName+"."+col.Name]; ok {
			return parseTypeString(repl)
		}
		// Wildcard match: "*.column"
		if repl, ok := m.columnReplacements["*."+col.Name]; ok {
			return parseTypeString(repl)
		}
	}

	// Check DB type replacements.
	if m.replacements != nil {
		if repl, ok := m.replacements[col.DBType]; ok {
			return parseTypeString(repl)
		}
		if col.EnumName != "" {
			if repl, ok := m.replacements[col.EnumName]; ok {
				return parseTypeString(repl)
			}
		}
	}

	// Enum columns get their generated enum type.
	if col.EnumName != "" {
		gt := GoType{Name: enumTypeName(col.EnumName)}
		if col.IsNullable {
			return m.wrapNull(gt)
		}
		return gt
	}

	base := mapDBType(col.DBType)

	if col.IsArray {
		return GoType{Name: "[]" + base.Name, Import: base.Import}
	}

	if col.IsNullable {
		return m.wrapNull(base)
	}

	return base
}

// wrapNull wraps a base type in the configured null wrapper.
func (m *TypeMapper) wrapNull(base GoType) GoType {
	switch m.nullType {
	case config.NullTypePointer:
		return GoType{Name: "*" + base.Name, Import: base.Import}
	case config.NullTypeDatabase:
		return mapToSQLNull(base)
	default: // NullTypeGeneric
		return GoType{
			Name:   fmt.Sprintf("runtime.Null[%s]", base.Name),
			Import: m.runtimePkg,
		}
	}
}

// mapDBType maps a normalized DB type name to its Go equivalent.
func mapDBType(dbType string) GoType {
	switch dbType {
	// Boolean
	case "boolean":
		return GoType{Name: "bool"}

	// Integer
	case "smallint":
		return GoType{Name: "int16"}
	case "integer", "mediumint":
		return GoType{Name: "int32"}
	case "bigint":
		return GoType{Name: "int64"}
	case "tinyint":
		return GoType{Name: "int8"}

	// Unsigned integer (MySQL)
	case "tinyint unsigned":
		return GoType{Name: "uint8"}
	case "smallint unsigned":
		return GoType{Name: "uint16"}
	case "integer unsigned", "mediumint unsigned":
		return GoType{Name: "uint32"}
	case "bigint unsigned":
		return GoType{Name: "uint64"}

	// Float
	case "real", "float":
		return GoType{Name: "float32"}
	case "double precision", "double":
		return GoType{Name: "float64"}
	case "numeric", "money", "decimal":
		return GoType{Name: "string"} // Numeric as string to avoid precision loss

	// String
	case "text", "character varying", "character", "name", "xml",
		"varchar", "char", "tinytext", "mediumtext", "longtext", "enum", "set":
		return GoType{Name: "string"}

	// Binary
	case "bytea", "blob", "tinyblob", "mediumblob", "longblob", "binary", "varbinary":
		return GoType{Name: "[]byte"}

	// Date/Time
	case "timestamp without time zone", "timestamp with time zone",
		"date", "time without time zone", "time with time zone",
		"datetime", "timestamp":
		return GoType{Name: "time.Time", Import: "time"}
	case "interval":
		return GoType{Name: "string"} // No native Go interval type
	case "time":
		return GoType{Name: "string"} // MySQL TIME has no direct Go equivalent
	case "year":
		return GoType{Name: "int16"}

	// UUID
	case "uuid":
		return GoType{Name: "string"} // Default to string; users can override via replacements

	// JSON
	case "json", "jsonb":
		return GoType{Name: "json.RawMessage", Import: "encoding/json"}

	// Network
	case "inet", "cidr", "macaddr":
		return GoType{Name: "string"}

	// Bit
	case "bit", "bit varying":
		return GoType{Name: "string"}

	// Full text search
	case "tsvector", "tsquery":
		return GoType{Name: "string"}

	// Range types
	case "int4range", "int8range", "numrange", "tsrange", "tstzrange", "daterange",
		"int4multirange", "int8multirange":
		return GoType{Name: "string"}

	default:
		return GoType{Name: "string"} // Fallback
	}
}

// mapToSQLNull maps a base type to its database/sql nullable equivalent.
func mapToSQLNull(base GoType) GoType {
	switch base.Name {
	case "string":
		return GoType{Name: "sql.NullString", Import: "database/sql"}
	case "int16":
		return GoType{Name: "sql.NullInt16", Import: "database/sql"}
	case "int32":
		return GoType{Name: "sql.NullInt32", Import: "database/sql"}
	case "int64":
		return GoType{Name: "sql.NullInt64", Import: "database/sql"}
	case "float32", "float64":
		return GoType{Name: "sql.NullFloat64", Import: "database/sql"}
	case "bool":
		return GoType{Name: "sql.NullBool", Import: "database/sql"}
	case "time.Time":
		return GoType{Name: "sql.NullTime", Import: "database/sql"}
	case "[]byte":
		return GoType{Name: "[]byte"} // []byte is already nullable
	default:
		return GoType{Name: "sql.NullString", Import: "database/sql"}
	}
}

// parseTypeString parses a fully qualified type like "github.com/google/uuid.UUID"
// into a GoType with the import path separated.
func parseTypeString(s string) GoType {
	// Find the last dot that's part of the package path (not the type separator).
	// "github.com/google/uuid.UUID" -> import="github.com/google/uuid", name="uuid.UUID"
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			// Check if what's before the dot looks like a package path.
			pkg := s[:i]
			typeName := s[i+1:]
			// Find the package short name from the import path.
			shortPkg := pkg
			for j := len(pkg) - 1; j >= 0; j-- {
				if pkg[j] == '/' {
					shortPkg = pkg[j+1:]
					break
				}
			}
			return GoType{Name: shortPkg + "." + typeName, Import: pkg}
		}
	}
	// No dot; it's a built-in type.
	return GoType{Name: s}
}

// enumTypeName converts a DB enum name to its Go type name.
func enumTypeName(name string) string {
	return naming.ToPascal(name)
}

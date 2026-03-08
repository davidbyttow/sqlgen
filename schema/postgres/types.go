// Package postgres implements a DDL parser for PostgreSQL using pg_query_go.
package postgres

import "strings"

// normalizeTypeName converts pg_catalog type names and aliases to canonical forms.
// pg_query returns types like ["pg_catalog", "int4"] for INTEGER, or ["serial"] for SERIAL.
func normalizeTypeName(names []string) (typeName string, isSerial bool) {
	if len(names) == 0 {
		return "unknown", false
	}

	// Take the last component (skip "pg_catalog" prefix if present).
	raw := names[len(names)-1]
	raw = strings.ToLower(raw)

	// Serial types are syntactic sugar for integer + sequence + default.
	switch raw {
	case "smallserial", "serial2":
		return "smallint", true
	case "serial", "serial4":
		return "integer", true
	case "bigserial", "serial8":
		return "bigint", true
	}

	// Map pg_catalog internal names to standard SQL names.
	mapped, ok := pgTypeMap[raw]
	if ok {
		return mapped, false
	}
	return raw, false
}

// pgTypeMap maps PostgreSQL internal type names to their canonical forms.
var pgTypeMap = map[string]string{
	// Numeric
	"int2":    "smallint",
	"int4":    "integer",
	"int8":    "bigint",
	"float4":  "real",
	"float8":  "double precision",
	"numeric": "numeric",
	"decimal": "numeric",
	"bool":    "boolean",

	// String
	"varchar": "character varying",
	"char":    "character",
	"bpchar":  "character",
	"text":    "text",
	"name":    "name",

	// Binary
	"bytea": "bytea",

	// Date/Time
	"timestamp":   "timestamp without time zone",
	"timestamptz": "timestamp with time zone",
	"date":        "date",
	"time":        "time without time zone",
	"timetz":      "time with time zone",
	"interval":    "interval",

	// UUID
	"uuid": "uuid",

	// JSON
	"json":  "json",
	"jsonb": "jsonb",

	// Network
	"inet":    "inet",
	"cidr":    "cidr",
	"macaddr": "macaddr",

	// Geometric
	"point":   "point",
	"line":    "line",
	"lseg":    "lseg",
	"box":     "box",
	"path":    "path",
	"polygon": "polygon",
	"circle":  "circle",

	// Range
	"int4range":   "int4range",
	"int8range":   "int8range",
	"numrange":    "numrange",
	"tsrange":     "tsrange",
	"tstzrange":   "tstzrange",
	"daterange":   "daterange",
	"int4multirange": "int4multirange",
	"int8multirange": "int8multirange",

	// Other
	"oid":      "oid",
	"xml":      "xml",
	"money":    "money",
	"tsvector": "tsvector",
	"tsquery":  "tsquery",
	"bit":      "bit",
	"varbit":   "bit varying",
}

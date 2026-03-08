package runtime

import "strconv"

// Dialect provides database-specific SQL generation behavior.
type Dialect interface {
	// Placeholder returns the parameter placeholder for the nth argument (1-indexed).
	// Postgres: $1, $2, ... MySQL: ?, SQLite: ?
	Placeholder(n int) string

	// QuoteIdent quotes an identifier (table name, column name).
	QuoteIdent(name string) string

	// SupportsReturning returns true if the dialect supports RETURNING clauses.
	SupportsReturning() bool
}

// PostgresDialect implements Dialect for PostgreSQL.
type PostgresDialect struct{}

func (PostgresDialect) Placeholder(n int) string {
	return "$" + strconv.Itoa(n)
}

func (PostgresDialect) QuoteIdent(name string) string {
	return `"` + name + `"`
}

func (PostgresDialect) SupportsReturning() bool {
	return true
}

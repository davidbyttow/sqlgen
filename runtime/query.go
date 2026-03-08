package runtime

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Executor is the interface for running queries. Satisfied by *sql.DB, *sql.Tx, etc.
type Executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// QueryMod is a function that modifies a query builder.
type QueryMod func(q *Query)

// Query builds a SQL query incrementally using composable mods.
type Query struct {
	dialect    Dialect
	table      string
	selectCols []string
	whereParts []wherePart
	orderBy    []string
	limit      *int
	offset     *int
	groupBy    []string
	having     []wherePart
	joins      []joinPart
	args       []any
}

type wherePart struct {
	clause string
	args   []any
}

type joinPart struct {
	joinType string // "JOIN", "LEFT JOIN", etc.
	table    string
	on       string
	args     []any
}

// NewQuery creates a new query builder for the given table.
func NewQuery(dialect Dialect, table string, mods ...QueryMod) *Query {
	q := &Query{
		dialect: dialect,
		table:   table,
	}
	for _, mod := range mods {
		mod(q)
	}
	return q
}

// Select specifies which columns to select. If not called, defaults to *.
func Select(cols ...string) QueryMod {
	return func(q *Query) {
		q.selectCols = append(q.selectCols, cols...)
	}
}

// Where adds a WHERE clause. Multiple calls are ANDed together.
// Use dialect-appropriate placeholders in the clause.
func Where(clause string, args ...any) QueryMod {
	return func(q *Query) {
		q.whereParts = append(q.whereParts, wherePart{clause: clause, args: args})
	}
}

// OrderBy adds ORDER BY clauses.
func OrderBy(cols ...string) QueryMod {
	return func(q *Query) {
		q.orderBy = append(q.orderBy, cols...)
	}
}

// Limit sets the LIMIT.
func Limit(n int) QueryMod {
	return func(q *Query) {
		q.limit = &n
	}
}

// Offset sets the OFFSET.
func Offset(n int) QueryMod {
	return func(q *Query) {
		q.offset = &n
	}
}

// GroupBy adds GROUP BY columns.
func GroupBy(cols ...string) QueryMod {
	return func(q *Query) {
		q.groupBy = append(q.groupBy, cols...)
	}
}

// Having adds a HAVING clause.
func Having(clause string, args ...any) QueryMod {
	return func(q *Query) {
		q.having = append(q.having, wherePart{clause: clause, args: args})
	}
}

// Join adds a JOIN clause.
func Join(table, on string, args ...any) QueryMod {
	return func(q *Query) {
		q.joins = append(q.joins, joinPart{joinType: "JOIN", table: table, on: on, args: args})
	}
}

// LeftJoin adds a LEFT JOIN clause.
func LeftJoin(table, on string, args ...any) QueryMod {
	return func(q *Query) {
		q.joins = append(q.joins, joinPart{joinType: "LEFT JOIN", table: table, on: on, args: args})
	}
}

// BuildSelect builds a SELECT query, returning the SQL string and args.
func (q *Query) BuildSelect() (string, []any) {
	var b strings.Builder
	var args []any
	argIdx := 0

	// SELECT
	b.WriteString("SELECT ")
	if len(q.selectCols) > 0 {
		b.WriteString(strings.Join(q.selectCols, ", "))
	} else {
		b.WriteString(q.dialect.QuoteIdent(q.table) + ".*")
	}

	// FROM
	b.WriteString(" FROM ")
	b.WriteString(q.dialect.QuoteIdent(q.table))

	// JOINs
	for _, j := range q.joins {
		b.WriteString(" ")
		b.WriteString(j.joinType)
		b.WriteString(" ")
		b.WriteString(j.table)
		b.WriteString(" ON ")
		clause, newArgs := q.rewritePlaceholders(j.on, j.args, &argIdx)
		b.WriteString(clause)
		args = append(args, newArgs...)
	}

	// WHERE
	if len(q.whereParts) > 0 {
		b.WriteString(" WHERE ")
		for i, wp := range q.whereParts {
			if i > 0 {
				b.WriteString(" AND ")
			}
			clause, newArgs := q.rewritePlaceholders(wp.clause, wp.args, &argIdx)
			b.WriteString(clause)
			args = append(args, newArgs...)
		}
	}

	// GROUP BY
	if len(q.groupBy) > 0 {
		b.WriteString(" GROUP BY ")
		b.WriteString(strings.Join(q.groupBy, ", "))
	}

	// HAVING
	if len(q.having) > 0 {
		b.WriteString(" HAVING ")
		for i, hp := range q.having {
			if i > 0 {
				b.WriteString(" AND ")
			}
			clause, newArgs := q.rewritePlaceholders(hp.clause, hp.args, &argIdx)
			b.WriteString(clause)
			args = append(args, newArgs...)
		}
	}

	// ORDER BY
	if len(q.orderBy) > 0 {
		b.WriteString(" ORDER BY ")
		b.WriteString(strings.Join(q.orderBy, ", "))
	}

	// LIMIT
	if q.limit != nil {
		argIdx++
		b.WriteString(" LIMIT ")
		b.WriteString(q.dialect.Placeholder(argIdx))
		args = append(args, *q.limit)
	}

	// OFFSET
	if q.offset != nil {
		argIdx++
		b.WriteString(" OFFSET ")
		b.WriteString(q.dialect.Placeholder(argIdx))
		args = append(args, *q.offset)
	}

	return b.String(), args
}

// BuildInsert builds an INSERT query with a RETURNING clause (if supported).
func BuildInsert(dialect Dialect, table string, cols []string, vals []any, returning []string) (string, []any) {
	var b strings.Builder
	b.WriteString("INSERT INTO ")
	b.WriteString(dialect.QuoteIdent(table))
	b.WriteString(" (")
	for i, col := range cols {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(dialect.QuoteIdent(col))
	}
	b.WriteString(") VALUES (")
	for i := range cols {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(dialect.Placeholder(i + 1))
	}
	b.WriteString(")")

	if len(returning) > 0 && dialect.SupportsReturning() {
		b.WriteString(" RETURNING ")
		for i, col := range returning {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(dialect.QuoteIdent(col))
		}
	}

	return b.String(), vals
}

// BuildUpdate builds an UPDATE query.
func BuildUpdate(dialect Dialect, table string, setCols []string, setVals []any, whereClauses []string, whereArgs []any) (string, []any) {
	var b strings.Builder
	var args []any

	b.WriteString("UPDATE ")
	b.WriteString(dialect.QuoteIdent(table))
	b.WriteString(" SET ")

	for i, col := range setCols {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(dialect.QuoteIdent(col))
		b.WriteString(" = ")
		b.WriteString(dialect.Placeholder(i + 1))
	}
	args = append(args, setVals...)

	if len(whereClauses) > 0 {
		b.WriteString(" WHERE ")
		offset := len(setCols)
		for i, clause := range whereClauses {
			if i > 0 {
				b.WriteString(" AND ")
			}
			// Rewrite ? placeholders to dialect-specific ones.
			b.WriteString(rewriteSinglePlaceholder(clause, dialect, offset+i+1))
		}
		args = append(args, whereArgs...)
	}

	return b.String(), args
}

// BuildDelete builds a DELETE query.
func BuildDelete(dialect Dialect, table string, whereClauses []string, whereArgs []any) (string, []any) {
	var b strings.Builder

	b.WriteString("DELETE FROM ")
	b.WriteString(dialect.QuoteIdent(table))

	if len(whereClauses) > 0 {
		b.WriteString(" WHERE ")
		for i, clause := range whereClauses {
			if i > 0 {
				b.WriteString(" AND ")
			}
			b.WriteString(rewriteSinglePlaceholder(clause, dialect, i+1))
		}
	}

	return b.String(), whereArgs
}

// BuildUpsert builds an INSERT ... ON CONFLICT DO UPDATE query (Postgres).
func BuildUpsert(dialect Dialect, table string, cols []string, vals []any, conflictCols []string, updateCols []string, returning []string) (string, []any) {
	var b strings.Builder
	b.WriteString("INSERT INTO ")
	b.WriteString(dialect.QuoteIdent(table))
	b.WriteString(" (")
	for i, col := range cols {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(dialect.QuoteIdent(col))
	}
	b.WriteString(") VALUES (")
	for i := range cols {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(dialect.Placeholder(i + 1))
	}
	b.WriteString(")")

	// ON CONFLICT
	b.WriteString(" ON CONFLICT (")
	for i, col := range conflictCols {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(dialect.QuoteIdent(col))
	}
	b.WriteString(") DO UPDATE SET ")

	for i, col := range updateCols {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(dialect.QuoteIdent(col))
		b.WriteString(" = EXCLUDED.")
		b.WriteString(dialect.QuoteIdent(col))
	}

	if len(returning) > 0 && dialect.SupportsReturning() {
		b.WriteString(" RETURNING ")
		for i, col := range returning {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(dialect.QuoteIdent(col))
		}
	}

	return b.String(), vals
}

// rewritePlaceholders replaces ? with dialect-specific placeholders, tracking global arg index.
func (q *Query) rewritePlaceholders(clause string, clauseArgs []any, argIdx *int) (string, []any) {
	if !strings.Contains(clause, "?") {
		// Already using dialect placeholders or no placeholders.
		return clause, clauseArgs
	}

	var b strings.Builder
	argI := 0
	for _, ch := range clause {
		if ch == '?' && argI < len(clauseArgs) {
			*argIdx++
			b.WriteString(q.dialect.Placeholder(*argIdx))
			argI++
		} else {
			b.WriteRune(ch)
		}
	}
	return b.String(), clauseArgs
}

// rewriteSinglePlaceholder replaces a single ? with a dialect placeholder.
func rewriteSinglePlaceholder(clause string, dialect Dialect, n int) string {
	return strings.Replace(clause, "?", dialect.Placeholder(n), 1)
}

// ExecQuery executes a query and returns the result.
func ExecQuery(ctx context.Context, exec Executor, query string, args ...any) (sql.Result, error) {
	return exec.ExecContext(ctx, query, args...)
}

// QueryRows executes a query and returns rows.
func QueryRows(ctx context.Context, exec Executor, query string, args ...any) (*sql.Rows, error) {
	return exec.QueryContext(ctx, query, args...)
}

// QueryRow executes a query expected to return at most one row.
func QueryRow(ctx context.Context, exec Executor, query string, args ...any) *sql.Row {
	return exec.QueryRowContext(ctx, query, args...)
}

// ErrNoRows is re-exported for convenience so generated code doesn't need to import database/sql.
var ErrNoRows = sql.ErrNoRows

// Count builds and executes a COUNT(*) query.
func Count(ctx context.Context, exec Executor, dialect Dialect, table string, mods ...QueryMod) (int64, error) {
	q := NewQuery(dialect, table, mods...)
	q.selectCols = []string{"COUNT(*)"}
	query, args := q.BuildSelect()

	var count int64
	err := exec.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

// Exists builds and executes an EXISTS query.
func Exists(ctx context.Context, exec Executor, dialect Dialect, table string, mods ...QueryMod) (bool, error) {
	q := NewQuery(dialect, table, mods...)
	q.selectCols = []string{"1"}
	q.limit = intPtr(1)
	inner, args := q.BuildSelect()

	query := fmt.Sprintf("SELECT EXISTS (%s)", inner)
	var exists bool
	err := exec.QueryRowContext(ctx, query, args...).Scan(&exists)
	return exists, err
}

func intPtr(n int) *int { return &n }

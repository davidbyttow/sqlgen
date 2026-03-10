package runtime

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"sort"
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
	distinct   bool
	distinctOn []string
	preloads   []PreloadDef
	ctes       []ctePart
	lock       *lockClause
}

type ctePart struct {
	name      string
	query     string
	args      []any
	recursive bool
}

type lockClause struct {
	strength string // "UPDATE", "SHARE", "NO KEY UPDATE", "KEY SHARE"
	nowait   bool
	skip     bool
}

type wherePart struct {
	clause      string
	args        []any
	conjunction string     // "AND" or "OR"
	group       []wherePart // if non-nil, this is a grouped expression
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
		q.whereParts = append(q.whereParts, wherePart{clause: clause, args: args, conjunction: "AND"})
	}
}

// Or adds a WHERE clause joined with OR instead of AND.
func Or(clause string, args ...any) QueryMod {
	return func(q *Query) {
		q.whereParts = append(q.whereParts, wherePart{clause: clause, args: args, conjunction: "OR"})
	}
}

// WhereIn adds a WHERE col IN ($1, $2, ...) clause.
func WhereIn(col string, vals ...any) QueryMod {
	return func(q *Query) {
		if len(vals) == 0 {
			return
		}
		placeholders := make([]string, len(vals))
		for i := range vals {
			placeholders[i] = "?"
		}
		clause := col + " IN (" + strings.Join(placeholders, ", ") + ")"
		q.whereParts = append(q.whereParts, wherePart{clause: clause, args: vals, conjunction: "AND"})
	}
}

// Expr creates a parenthesized group of conditions from the given mods.
// Only WHERE-related mods (Where, Or) are meaningful inside an Expr.
func Expr(mods ...QueryMod) QueryMod {
	return func(q *Query) {
		// Build a temporary query to collect the where parts.
		tmp := &Query{}
		for _, mod := range mods {
			mod(tmp)
		}
		if len(tmp.whereParts) > 0 {
			q.whereParts = append(q.whereParts, wherePart{
				conjunction: "AND",
				group:       tmp.whereParts,
			})
		}
	}
}

// Distinct adds DISTINCT to the SELECT clause.
func Distinct() QueryMod {
	return func(q *Query) {
		q.distinct = true
	}
}

// DistinctOn adds DISTINCT ON (cols...) to the SELECT clause (Postgres-specific).
func DistinctOn(cols ...string) QueryMod {
	return func(q *Query) {
		q.distinctOn = cols
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

// RightJoin adds a RIGHT JOIN clause.
func RightJoin(table, on string, args ...any) QueryMod {
	return func(q *Query) {
		q.joins = append(q.joins, joinPart{joinType: "RIGHT JOIN", table: table, on: on, args: args})
	}
}

// FullJoin adds a FULL JOIN clause.
func FullJoin(table, on string, args ...any) QueryMod {
	return func(q *Query) {
		q.joins = append(q.joins, joinPart{joinType: "FULL JOIN", table: table, on: on, args: args})
	}
}

// CrossJoin adds a CROSS JOIN clause (no ON condition).
func CrossJoin(table string) QueryMod {
	return func(q *Query) {
		q.joins = append(q.joins, joinPart{joinType: "CROSS JOIN", table: table})
	}
}

// WithCTE adds a Common Table Expression (WITH clause) to the query.
// The query string should be a complete SELECT statement.
func WithCTE(name string, query string, args ...any) QueryMod {
	return func(q *Query) {
		q.ctes = append(q.ctes, ctePart{name: name, query: query, args: args})
	}
}

// WithRecursiveCTE adds a recursive CTE (WITH RECURSIVE) to the query.
func WithRecursiveCTE(name string, query string, args ...any) QueryMod {
	return func(q *Query) {
		q.ctes = append(q.ctes, ctePart{name: name, query: query, args: args, recursive: true})
	}
}

// ForUpdate adds FOR UPDATE row locking to the query.
func ForUpdate() QueryMod {
	return func(q *Query) {
		if q.lock == nil {
			q.lock = &lockClause{}
		}
		q.lock.strength = "UPDATE"
	}
}

// ForShare adds FOR SHARE row locking to the query.
func ForShare() QueryMod {
	return func(q *Query) {
		if q.lock == nil {
			q.lock = &lockClause{}
		}
		q.lock.strength = "SHARE"
	}
}

// ForNoKeyUpdate adds FOR NO KEY UPDATE row locking to the query.
func ForNoKeyUpdate() QueryMod {
	return func(q *Query) {
		if q.lock == nil {
			q.lock = &lockClause{}
		}
		q.lock.strength = "NO KEY UPDATE"
	}
}

// ForKeyShare adds FOR KEY SHARE row locking to the query.
func ForKeyShare() QueryMod {
	return func(q *Query) {
		if q.lock == nil {
			q.lock = &lockClause{}
		}
		q.lock.strength = "KEY SHARE"
	}
}

// Nowait adds NOWAIT to the row locking clause. Must be used with ForUpdate/ForShare.
func Nowait() QueryMod {
	return func(q *Query) {
		if q.lock == nil {
			q.lock = &lockClause{}
		}
		q.lock.nowait = true
	}
}

// SkipLocked adds SKIP LOCKED to the row locking clause. Must be used with ForUpdate/ForShare.
func SkipLocked() QueryMod {
	return func(q *Query) {
		if q.lock == nil {
			q.lock = &lockClause{}
		}
		q.lock.skip = true
	}
}

// Preloads returns the preload definitions registered on this query.
func (q *Query) Preloads() []PreloadDef {
	return q.preloads
}

// BuildSelect builds a SELECT query, returning the SQL string and args.
func (q *Query) BuildSelect() (string, []any) {
	var b strings.Builder
	var args []any
	argIdx := 0

	// CTEs (WITH clause)
	if len(q.ctes) > 0 {
		recursive := false
		for _, cte := range q.ctes {
			if cte.recursive {
				recursive = true
				break
			}
		}
		if recursive {
			b.WriteString("WITH RECURSIVE ")
		} else {
			b.WriteString("WITH ")
		}
		for i, cte := range q.ctes {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(q.dialect.QuoteIdent(cte.name))
			b.WriteString(" AS (")
			clause, newArgs := q.rewritePlaceholders(cte.query, cte.args, &argIdx)
			b.WriteString(clause)
			args = append(args, newArgs...)
			b.WriteString(")")
		}
		b.WriteString(" ")
	}

	// SELECT
	b.WriteString("SELECT ")
	if len(q.distinctOn) > 0 {
		b.WriteString("DISTINCT ON (")
		b.WriteString(strings.Join(q.distinctOn, ", "))
		b.WriteString(") ")
	} else if q.distinct {
		b.WriteString("DISTINCT ")
	}
	if len(q.selectCols) > 0 {
		b.WriteString(strings.Join(q.selectCols, ", "))
	} else {
		b.WriteString(q.dialect.QuoteIdent(q.table) + ".*")
	}

	// Preload columns (appended after base columns)
	for _, p := range q.preloads {
		for _, col := range p.Columns {
			b.WriteString(", ")
			b.WriteString(col)
		}
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
		if j.on != "" {
			b.WriteString(" ON ")
			clause, newArgs := q.rewritePlaceholders(j.on, j.args, &argIdx)
			b.WriteString(clause)
			args = append(args, newArgs...)
		}
	}

	// Preload LEFT JOINs
	for _, p := range q.preloads {
		b.WriteString(" LEFT JOIN ")
		b.WriteString(q.dialect.QuoteIdent(p.Table))
		b.WriteString(" ON ")
		b.WriteString(p.JoinCond)
	}

	// WHERE
	if len(q.whereParts) > 0 {
		b.WriteString(" WHERE ")
		q.renderWhereParts(&b, q.whereParts, &argIdx, &args)
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

	// Row locking (FOR UPDATE/SHARE/etc.)
	if q.lock != nil && q.lock.strength != "" {
		b.WriteString(" FOR ")
		b.WriteString(q.lock.strength)
		if q.lock.nowait {
			b.WriteString(" NOWAIT")
		} else if q.lock.skip {
			b.WriteString(" SKIP LOCKED")
		}
	}

	return b.String(), args
}

// BuildUpdateAll builds an UPDATE ... SET ... WHERE ... query using the query's where parts.
func (q *Query) BuildUpdateAll(set map[string]any) (string, []any) {
	var b strings.Builder
	var args []any
	argIdx := 0

	b.WriteString("UPDATE ")
	b.WriteString(q.dialect.QuoteIdent(q.table))
	b.WriteString(" SET ")

	// Sort keys for deterministic output.
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i, col := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		argIdx++
		b.WriteString(q.dialect.QuoteIdent(col))
		b.WriteString(" = ")
		b.WriteString(q.dialect.Placeholder(argIdx))
		args = append(args, set[col])
	}

	if len(q.whereParts) > 0 {
		b.WriteString(" WHERE ")
		q.renderWhereParts(&b, q.whereParts, &argIdx, &args)
	}

	return b.String(), args
}

// BuildDeleteAll builds a DELETE FROM ... WHERE ... query using the query's where parts.
func (q *Query) BuildDeleteAll() (string, []any) {
	var b strings.Builder
	var args []any
	argIdx := 0

	b.WriteString("DELETE FROM ")
	b.WriteString(q.dialect.QuoteIdent(q.table))

	if len(q.whereParts) > 0 {
		b.WriteString(" WHERE ")
		q.renderWhereParts(&b, q.whereParts, &argIdx, &args)
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

// renderWhereParts writes a list of where parts to the builder, handling conjunctions and groups.
func (q *Query) renderWhereParts(b *strings.Builder, parts []wherePart, argIdx *int, args *[]any) {
	for i, wp := range parts {
		if i > 0 {
			b.WriteString(" ")
			b.WriteString(wp.conjunction)
			b.WriteString(" ")
		}
		if wp.group != nil {
			b.WriteString("(")
			q.renderWhereParts(b, wp.group, argIdx, args)
			b.WriteString(")")
		} else {
			clause, newArgs := q.rewritePlaceholders(wp.clause, wp.args, argIdx)
			b.WriteString(clause)
			*args = append(*args, newArgs...)
		}
	}
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

// RawQueryResult holds a raw SQL query and its arguments.
type RawQueryResult struct {
	query string
	args  []any
}

// RawSQL creates a RawQueryResult from a raw SQL string and arguments.
func RawSQL(query string, args ...any) *RawQueryResult {
	return &RawQueryResult{query: query, args: args}
}

// SQL returns the raw SQL query and its arguments.
func (r *RawQueryResult) SQL() (string, []any) {
	return r.query, r.args
}

// Exec executes the raw query.
func (r *RawQueryResult) Exec(ctx context.Context, exec Executor) (sql.Result, error) {
	return exec.ExecContext(ctx, r.query, r.args...)
}

// QueryRows executes the raw query and returns rows.
func (r *RawQueryResult) QueryRows(ctx context.Context, exec Executor) (*sql.Rows, error) {
	return exec.QueryContext(ctx, r.query, r.args...)
}

// QueryRow executes the raw query and returns a single row.
func (r *RawQueryResult) QueryRow(ctx context.Context, exec Executor) *sql.Row {
	return exec.QueryRowContext(ctx, r.query, r.args...)
}

// DebugExecutor wraps an Executor and prints SQL before executing.
type DebugExecutor struct {
	Exec   Executor
	Writer io.Writer
}

// Debug creates a DebugExecutor that logs to os.Stderr.
func Debug(exec Executor) *DebugExecutor {
	return &DebugExecutor{Exec: exec, Writer: os.Stderr}
}

// DebugTo creates a DebugExecutor that logs to the given writer.
func DebugTo(exec Executor, w io.Writer) *DebugExecutor {
	return &DebugExecutor{Exec: exec, Writer: w}
}

func (d *DebugExecutor) logQuery(query string, args ...any) {
	fmt.Fprintf(d.Writer, "SQL: %s\nArgs: %v\n", query, args)
}

// ExecContext implements Executor.
func (d *DebugExecutor) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	d.logQuery(query, args...)
	return d.Exec.ExecContext(ctx, query, args...)
}

// QueryContext implements Executor.
func (d *DebugExecutor) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	d.logQuery(query, args...)
	return d.Exec.QueryContext(ctx, query, args...)
}

// QueryRowContext implements Executor.
func (d *DebugExecutor) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	d.logQuery(query, args...)
	return d.Exec.QueryRowContext(ctx, query, args...)
}

package runtime

import (
	"context"
	"database/sql"
	"strings"
)

// Loader handles eager loading of relationships via separate queries.
// Generated code creates specific loaders per relationship; this provides the building blocks.

// EagerLoadRequest represents a request to eagerly load a relationship.
type EagerLoadRequest struct {
	Name   string             // Relationship name (e.g., "Posts", "User")
	Mods   []QueryMod         // Optional mods to apply to the loading query
	Nested []*EagerLoadRequest // Nested loads (e.g., "Posts.Tags")
}

// Load creates an EagerLoadRequest. Supports dot-notation for nested loading.
// Optional QueryMods filter the loaded relationship.
func Load(name string, mods ...QueryMod) *EagerLoadRequest {
	parts := strings.SplitN(name, ".", 2)
	req := &EagerLoadRequest{
		Name: parts[0],
		Mods: mods,
	}
	if len(parts) > 1 {
		// Nested: "Posts.Tags" -> Load("Posts") with nested Load("Tags")
		// Mods only apply to the leaf when using dot notation.
		req.Mods = nil
		req.Nested = []*EagerLoadRequest{{Name: parts[1], Mods: mods}}
	}
	return req
}

// LoadFunc is the signature for generated loader functions.
// It takes the parent models, exec, dialect, and loads, then populates .R fields.
type LoadFunc func(ctx context.Context, exec Executor, dialect Dialect, parentModels any, loads []*EagerLoadRequest) error

// LoadMany executes a query to load related records for a set of parent PKs.
// It returns the raw rows; generated code handles scanning and assignment.
// Optional mods are applied to the query (e.g., Where, OrderBy, Limit).
func LoadMany(ctx context.Context, exec Executor, dialect Dialect, table string, fkCol string, parentIDs []any, mods ...QueryMod) (*sql.Rows, error) {
	if len(parentIDs) == 0 {
		return nil, nil
	}

	q := NewQuery(dialect, table, mods...)
	q.whereParts = append([]wherePart{{
		clause:      buildInClause(dialect, fkCol, len(parentIDs)),
		args:        parentIDs,
		conjunction: "AND",
	}}, q.whereParts...)

	query, args := q.BuildSelect()
	return exec.QueryContext(ctx, query, args...)
}

// buildInClause constructs a "col" IN (?, ?, ...) clause with ? placeholders
// that get renumbered by the query builder's rewritePlaceholders.
func buildInClause(dialect Dialect, col string, n int) string {
	return buildInClauseWithPrefix(dialect, "", col, n)
}

// buildInClauseWithPrefix constructs a prefix."col" IN (?, ?, ...) clause.
func buildInClauseWithPrefix(dialect Dialect, prefix, col string, n int) string {
	var b strings.Builder
	if prefix != "" {
		b.WriteString(prefix)
		b.WriteString(".")
	}
	b.WriteString(dialect.QuoteIdent(col))
	b.WriteString(" IN (")
	for i := range n {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("?")
	}
	b.WriteString(")")
	return b.String()
}

// LoadOne executes a query to load a single related record by FK value.
func LoadOne(ctx context.Context, exec Executor, dialect Dialect, table string, fkCol string, fkVal any) (*sql.Row, error) {
	var b strings.Builder
	b.WriteString("SELECT * FROM ")
	b.WriteString(dialect.QuoteIdent(table))
	b.WriteString(" WHERE ")
	b.WriteString(dialect.QuoteIdent(fkCol))
	b.WriteString(" = ")
	b.WriteString(dialect.Placeholder(1))
	b.WriteString(" LIMIT 1")

	return exec.QueryRowContext(ctx, b.String(), fkVal), nil
}

// LoadManyToMany executes a query to load related records through a join table.
// Optional mods are applied to filter the target records.
func LoadManyToMany(ctx context.Context, exec Executor, dialect Dialect, targetTable, joinTable, joinLocalCol, joinForeignCol, targetPKCol string, localIDs []any, mods ...QueryMod) (*sql.Rows, string, error) {
	if len(localIDs) == 0 {
		return nil, "", nil
	}

	joinAlias := "__jt"
	joinKeyAlias := "__join_key"

	// Build using Query so user mods (Where, OrderBy, Limit, etc.) apply naturally.
	q := NewQuery(dialect, targetTable, mods...)
	q.selectCols = append([]string{
		dialect.QuoteIdent(targetTable) + ".*",
		joinAlias + "." + dialect.QuoteIdent(joinLocalCol) + " AS " + joinKeyAlias,
	}, q.selectCols...)
	q.joins = append([]joinPart{{
		joinType: "JOIN",
		table:    dialect.QuoteIdent(joinTable) + " AS " + joinAlias,
		on:       joinAlias + "." + dialect.QuoteIdent(joinForeignCol) + " = " + dialect.QuoteIdent(targetTable) + "." + dialect.QuoteIdent(targetPKCol),
	}}, q.joins...)
	q.whereParts = append([]wherePart{{
		clause:      buildInClauseWithPrefix(dialect, joinAlias, joinLocalCol, len(localIDs)),
		args:        localIDs,
		conjunction: "AND",
	}}, q.whereParts...)

	query, args := q.BuildSelect()
	rows, err := exec.QueryContext(ctx, query, args...)
	return rows, joinKeyAlias, err
}

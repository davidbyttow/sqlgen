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
func LoadMany(ctx context.Context, exec Executor, dialect Dialect, table string, fkCol string, parentIDs []any) (*sql.Rows, error) {
	if len(parentIDs) == 0 {
		return nil, nil
	}

	var b strings.Builder
	b.WriteString("SELECT * FROM ")
	b.WriteString(dialect.QuoteIdent(table))
	b.WriteString(" WHERE ")
	b.WriteString(dialect.QuoteIdent(fkCol))
	b.WriteString(" IN (")
	for i := range parentIDs {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(dialect.Placeholder(i + 1))
	}
	b.WriteString(")")

	return exec.QueryContext(ctx, b.String(), parentIDs...)
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
func LoadManyToMany(ctx context.Context, exec Executor, dialect Dialect, targetTable, joinTable, joinLocalCol, joinForeignCol, targetPKCol string, localIDs []any) (*sql.Rows, string, error) {
	if len(localIDs) == 0 {
		return nil, "", nil
	}

	// SELECT target.*, join_table.local_col AS __join_key
	// FROM target
	// JOIN join_table ON join_table.foreign_col = target.pk_col
	// WHERE join_table.local_col IN ($1, $2, ...)

	joinAlias := "__jt"
	joinKeyAlias := "__join_key"

	var b strings.Builder
	b.WriteString("SELECT ")
	b.WriteString(dialect.QuoteIdent(targetTable))
	b.WriteString(".*, ")
	b.WriteString(joinAlias)
	b.WriteString(".")
	b.WriteString(dialect.QuoteIdent(joinLocalCol))
	b.WriteString(" AS ")
	b.WriteString(joinKeyAlias)

	b.WriteString(" FROM ")
	b.WriteString(dialect.QuoteIdent(targetTable))

	b.WriteString(" JOIN ")
	b.WriteString(dialect.QuoteIdent(joinTable))
	b.WriteString(" AS ")
	b.WriteString(joinAlias)
	b.WriteString(" ON ")
	b.WriteString(joinAlias)
	b.WriteString(".")
	b.WriteString(dialect.QuoteIdent(joinForeignCol))
	b.WriteString(" = ")
	b.WriteString(dialect.QuoteIdent(targetTable))
	b.WriteString(".")
	b.WriteString(dialect.QuoteIdent(targetPKCol))

	b.WriteString(" WHERE ")
	b.WriteString(joinAlias)
	b.WriteString(".")
	b.WriteString(dialect.QuoteIdent(joinLocalCol))
	b.WriteString(" IN (")
	for i := range localIDs {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(dialect.Placeholder(i + 1))
	}
	b.WriteString(")")

	rows, err := exec.QueryContext(ctx, b.String(), localIDs...)
	return rows, joinKeyAlias, err
}

package sqlgen

import (
	"testing"
)

func TestEachAndCursorCompile(t *testing.T) {
	// Each and Cursor use generics constrained by RowScanner.
	// We can't test actual DB iteration without a connection,
	// but we verify the types compile and the query builds correctly.
	d := PostgresDialect{}
	q := NewQuery(d, "users", Limit(10))
	query, args := q.BuildSelect()

	if query != `SELECT "users".* FROM "users" LIMIT $1` {
		t.Errorf("query = %q", query)
	}
	if len(args) != 1 || args[0] != 10 {
		t.Errorf("args = %v", args)
	}
}

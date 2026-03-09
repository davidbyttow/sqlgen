package runtime

import (
	"bytes"
	"context"
	"database/sql"
	"testing"
)

func TestBuildSelectSimple(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "users")
	sql, args := q.BuildSelect()

	want := `SELECT "users".* FROM "users"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 0 {
		t.Errorf("args = %v, want empty", args)
	}
}

func TestBuildSelectWithMods(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "users",
		Select(`"users"."id"`, `"users"."name"`),
		Where(`"users"."age" > ?`, 30),
		Where(`"users"."active" = ?`, true),
		OrderBy(`"users"."name" ASC`),
		Limit(10),
		Offset(20),
	)
	sql, args := q.BuildSelect()

	want := `SELECT "users"."id", "users"."name" FROM "users" WHERE "users"."age" > $1 AND "users"."active" = $2 ORDER BY "users"."name" ASC LIMIT $3 OFFSET $4`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 4 {
		t.Errorf("args count = %d, want 4", len(args))
	}
	if args[0] != 30 || args[1] != true || args[2] != 10 || args[3] != 20 {
		t.Errorf("args = %v", args)
	}
}

func TestBuildSelectGroupByHaving(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "orders",
		Select("customer_id", "COUNT(*) as cnt"),
		GroupBy("customer_id"),
		Having("COUNT(*) > ?", 5),
	)
	sql, args := q.BuildSelect()

	want := `SELECT customer_id, COUNT(*) as cnt FROM "orders" GROUP BY customer_id HAVING COUNT(*) > $1`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 1 || args[0] != 5 {
		t.Errorf("args = %v", args)
	}
}

func TestBuildSelectJoin(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "posts",
		LeftJoin(`"users"`, `"users"."id" = "posts"."author_id"`),
	)
	sql, _ := q.BuildSelect()

	want := `SELECT "posts".* FROM "posts" LEFT JOIN "users" ON "users"."id" = "posts"."author_id"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
}

func TestBuildInsert(t *testing.T) {
	d := PostgresDialect{}
	sql, args := BuildInsert(d, "users",
		[]string{"name", "email"},
		[]any{"Alice", "alice@example.com"},
		[]string{"id", "created_at"},
	)

	want := `INSERT INTO "users" ("name", "email") VALUES ($1, $2) RETURNING "id", "created_at"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 2 {
		t.Errorf("args = %v", args)
	}
}

func TestBuildInsertNoReturning(t *testing.T) {
	// Simulate a dialect that doesn't support RETURNING.
	d := testDialectNoReturning{}
	sql, _ := BuildInsert(d, "users",
		[]string{"name"},
		[]any{"Alice"},
		[]string{"id"},
	)

	if sql != `INSERT INTO "users" ("name") VALUES (?)` {
		t.Errorf("got: %s", sql)
	}
}

func TestBuildUpdate(t *testing.T) {
	d := PostgresDialect{}
	sql, args := BuildUpdate(d, "users",
		[]string{"name", "email"},
		[]any{"Alice", "alice@example.com"},
		[]string{`"id" = ?`},
		[]any{"some-uuid"},
	)

	want := `UPDATE "users" SET "name" = $1, "email" = $2 WHERE "id" = $3`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 3 {
		t.Errorf("args = %v", args)
	}
}

func TestBuildDelete(t *testing.T) {
	d := PostgresDialect{}
	sql, args := BuildDelete(d, "users",
		[]string{`"id" = ?`},
		[]any{"some-uuid"},
	)

	want := `DELETE FROM "users" WHERE "id" = $1`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 1 {
		t.Errorf("args = %v", args)
	}
}

func TestBuildUpsert(t *testing.T) {
	d := PostgresDialect{}
	sql, args := BuildUpsert(d, "users",
		[]string{"id", "name", "email"},
		[]any{"uuid-1", "Alice", "alice@example.com"},
		[]string{"id"},
		[]string{"name", "email"},
		[]string{"id", "name", "email"},
	)

	want := `INSERT INTO "users" ("id", "name", "email") VALUES ($1, $2, $3) ON CONFLICT ("id") DO UPDATE SET "name" = EXCLUDED."name", "email" = EXCLUDED."email" RETURNING "id", "name", "email"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 3 {
		t.Errorf("args = %v", args)
	}
}

func TestPostgresDialect(t *testing.T) {
	d := PostgresDialect{}

	if d.Placeholder(1) != "$1" {
		t.Errorf("Placeholder(1) = %q", d.Placeholder(1))
	}
	if d.Placeholder(10) != "$10" {
		t.Errorf("Placeholder(10) = %q", d.Placeholder(10))
	}
	if d.QuoteIdent("users") != `"users"` {
		t.Errorf("QuoteIdent = %q", d.QuoteIdent("users"))
	}
	if !d.SupportsReturning() {
		t.Error("Postgres should support RETURNING")
	}
}

// testDialectNoReturning is a test dialect that doesn't support RETURNING.
type testDialectNoReturning struct{}

func (testDialectNoReturning) Placeholder(n int) string    { return "?" }
func (testDialectNoReturning) QuoteIdent(name string) string { return `"` + name + `"` }
func (testDialectNoReturning) SupportsReturning() bool      { return false }

func TestBuildSelectOr(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "users",
		Where("a = ?", 1),
		Or("b = ?", 2),
	)
	sql, args := q.BuildSelect()

	want := `SELECT "users".* FROM "users" WHERE a = $1 OR b = $2`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 2 || args[0] != 1 || args[1] != 2 {
		t.Errorf("args = %v", args)
	}
}

func TestBuildSelectMixedAndOr(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "users",
		Where("a = ?", 1),
		Where("b = ?", 2),
		Or("c = ?", 3),
	)
	sql, args := q.BuildSelect()

	want := `SELECT "users".* FROM "users" WHERE a = $1 AND b = $2 OR c = $3`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 3 {
		t.Errorf("args = %v", args)
	}
}

func TestBuildSelectExprGrouping(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "users",
		Expr(Where("a = ?", 1), Or("b = ?", 2)),
		Where("c = ?", 3),
	)
	sql, args := q.BuildSelect()

	want := `SELECT "users".* FROM "users" WHERE (a = $1 OR b = $2) AND c = $3`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 3 {
		t.Errorf("args = %v", args)
	}
}

func TestBuildSelectNestedExpr(t *testing.T) {
	d := PostgresDialect{}
	// (a = 1 OR b = 2) OR (c = 3 OR d = 4)
	q := NewQuery(d, "users",
		Expr(Where("a = ?", 1), Or("b = ?", 2)),
	)
	// Add the second Expr with OR conjunction manually.
	// We need to use a slightly different approach: wrap the second Expr's group with OR.
	orExpr := func(mods ...QueryMod) QueryMod {
		return func(q *Query) {
			tmp := &Query{}
			for _, mod := range mods {
				mod(tmp)
			}
			if len(tmp.whereParts) > 0 {
				q.whereParts = append(q.whereParts, wherePart{
					conjunction: "OR",
					group:       tmp.whereParts,
				})
			}
		}
	}
	orExpr(Where("c = ?", 3), Or("d = ?", 4))(q)

	sql, args := q.BuildSelect()

	want := `SELECT "users".* FROM "users" WHERE (a = $1 OR b = $2) OR (c = $3 OR d = $4)`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 4 {
		t.Errorf("args = %v", args)
	}
}

func TestBuildSelectRightJoin(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "posts",
		RightJoin(`"users"`, `"users"."id" = "posts"."author_id"`),
	)
	sql, _ := q.BuildSelect()

	want := `SELECT "posts".* FROM "posts" RIGHT JOIN "users" ON "users"."id" = "posts"."author_id"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
}

func TestBuildSelectFullJoin(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "posts",
		FullJoin(`"users"`, `"users"."id" = "posts"."author_id"`),
	)
	sql, _ := q.BuildSelect()

	want := `SELECT "posts".* FROM "posts" FULL JOIN "users" ON "users"."id" = "posts"."author_id"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
}

func TestBuildSelectCrossJoin(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "posts",
		CrossJoin(`"categories"`),
	)
	sql, _ := q.BuildSelect()

	want := `SELECT "posts".* FROM "posts" CROSS JOIN "categories"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
}

func TestBuildSelectDistinct(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "users",
		Distinct(),
		Select("name"),
	)
	sql, _ := q.BuildSelect()

	want := `SELECT DISTINCT name FROM "users"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
}

func TestBuildSelectDistinctOn(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "users",
		DistinctOn("name", "email"),
		Select("name", "email", "age"),
	)
	sql, _ := q.BuildSelect()

	want := `SELECT DISTINCT ON (name, email) name, email, age FROM "users"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
}

func TestRawSQL(t *testing.T) {
	r := RawSQL("SELECT * FROM users WHERE id = $1", 42)
	query, args := r.SQL()

	if query != "SELECT * FROM users WHERE id = $1" {
		t.Errorf("query = %q", query)
	}
	if len(args) != 1 || args[0] != 42 {
		t.Errorf("args = %v", args)
	}
}

func TestDebugExecutor(t *testing.T) {
	var buf bytes.Buffer
	mock := &mockExecutor{}
	de := DebugTo(mock, &buf)

	// Verify it implements Executor by calling ExecContext.
	_, _ = de.ExecContext(nil, "SELECT 1", "arg1") //nolint:staticcheck

	output := buf.String()
	if output == "" {
		t.Error("expected debug output, got empty string")
	}
	if !bytes.Contains([]byte(output), []byte("SELECT 1")) {
		t.Errorf("output missing query: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("arg1")) {
		t.Errorf("output missing args: %s", output)
	}
}

func TestWhereIn(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "users",
		WhereIn(`"id"`, "a", "b", "c"),
	)
	sql, args := q.BuildSelect()

	want := `SELECT "users".* FROM "users" WHERE "id" IN ($1, $2, $3)`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 3 || args[0] != "a" || args[1] != "b" || args[2] != "c" {
		t.Errorf("args = %v", args)
	}
}

func TestWhereInEmpty(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "users",
		WhereIn(`"id"`),
	)
	sql, args := q.BuildSelect()

	want := `SELECT "users".* FROM "users"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 0 {
		t.Errorf("args = %v, want empty", args)
	}
}

func TestBuildUpdateAll(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "users",
		Where(`"active" = ?`, true),
	)
	sql, args := q.BuildUpdateAll(map[string]any{
		"name":  "updated",
		"email": "new@example.com",
	})

	want := `UPDATE "users" SET "email" = $1, "name" = $2 WHERE "active" = $3`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 3 || args[0] != "new@example.com" || args[1] != "updated" || args[2] != true {
		t.Errorf("args = %v", args)
	}
}

func TestBuildUpdateAllNoWhere(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "users")
	sql, args := q.BuildUpdateAll(map[string]any{
		"active": false,
	})

	want := `UPDATE "users" SET "active" = $1`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 1 || args[0] != false {
		t.Errorf("args = %v", args)
	}
}

func TestBuildDeleteAll(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "users",
		Where(`"active" = ?`, false),
	)
	sql, args := q.BuildDeleteAll()

	want := `DELETE FROM "users" WHERE "active" = $1`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 1 || args[0] != false {
		t.Errorf("args = %v", args)
	}
}

func TestBuildDeleteAllNoWhere(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "users")
	sql, args := q.BuildDeleteAll()

	want := `DELETE FROM "users"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 0 {
		t.Errorf("args = %v, want empty", args)
	}
}

func TestBuildUpdateAllWithWhereIn(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "users",
		WhereIn(`"id"`, 1, 2, 3),
	)
	sql, args := q.BuildUpdateAll(map[string]any{
		"active": true,
	})

	want := `UPDATE "users" SET "active" = $1 WHERE "id" IN ($2, $3, $4)`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 4 || args[0] != true || args[1] != 1 || args[2] != 2 || args[3] != 3 {
		t.Errorf("args = %v", args)
	}
}

func TestBuildDeleteAllWithWhereIn(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "users",
		WhereIn(`"id"`, 1, 2, 3),
	)
	sql, args := q.BuildDeleteAll()

	want := `DELETE FROM "users" WHERE "id" IN ($1, $2, $3)`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 3 || args[0] != 1 || args[1] != 2 || args[2] != 3 {
		t.Errorf("args = %v", args)
	}
}

// mockExecutor is a no-op Executor for testing DebugExecutor.
type mockExecutor struct{}

func (m *mockExecutor) ExecContext(_ context.Context, _ string, _ ...any) (sql.Result, error) {
	return nil, nil
}
func (m *mockExecutor) QueryContext(_ context.Context, _ string, _ ...any) (*sql.Rows, error) {
	return nil, nil
}
func (m *mockExecutor) QueryRowContext(_ context.Context, _ string, _ ...any) *sql.Row {
	return nil
}

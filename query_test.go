package sqlgen

import (
	"bytes"
	"context"
	"database/sql"
	"strings"
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

func TestBuildSelectCTE(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "employees",
		WithCTE("managers", "SELECT * FROM employees WHERE role = ?", "manager"),
		Where(`"department" = ?`, "engineering"),
	)
	sql, args := q.BuildSelect()

	want := `WITH "managers" AS (SELECT * FROM employees WHERE role = $1) SELECT "employees".* FROM "employees" WHERE "department" = $2`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 2 || args[0] != "manager" || args[1] != "engineering" {
		t.Errorf("args = %v", args)
	}
}

func TestBuildSelectMultipleCTEs(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "combined",
		WithCTE("active_users", "SELECT * FROM users WHERE active = ?", true),
		WithCTE("recent_posts", "SELECT * FROM posts WHERE created_at > ?", "2024-01-01"),
	)
	sql, args := q.BuildSelect()

	want := `WITH "active_users" AS (SELECT * FROM users WHERE active = $1), "recent_posts" AS (SELECT * FROM posts WHERE created_at > $2) SELECT "combined".* FROM "combined"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 2 {
		t.Errorf("args = %v", args)
	}
}

func TestBuildSelectRecursiveCTE(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "tree",
		WithRecursiveCTE("tree", "SELECT id, parent_id, name FROM categories WHERE parent_id IS NULL UNION ALL SELECT c.id, c.parent_id, c.name FROM categories c JOIN tree t ON c.parent_id = t.id"),
	)
	sql, args := q.BuildSelect()

	want := `WITH RECURSIVE "tree" AS (SELECT id, parent_id, name FROM categories WHERE parent_id IS NULL UNION ALL SELECT c.id, c.parent_id, c.name FROM categories c JOIN tree t ON c.parent_id = t.id) SELECT "tree".* FROM "tree"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 0 {
		t.Errorf("args = %v, want empty", args)
	}
}

func TestBuildSelectMixedCTEs(t *testing.T) {
	d := PostgresDialect{}
	// If any CTE is recursive, the whole WITH block gets RECURSIVE.
	q := NewQuery(d, "result",
		WithCTE("base", "SELECT 1"),
		WithRecursiveCTE("tree", "SELECT 1 UNION ALL SELECT 1"),
	)
	sql, _ := q.BuildSelect()

	if !strings.Contains(sql, "WITH RECURSIVE") {
		t.Errorf("expected WITH RECURSIVE, got: %s", sql)
	}
}

func TestBuildSelectForUpdate(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "accounts",
		Where(`"id" = ?`, 1),
		ForUpdate(),
	)
	sql, args := q.BuildSelect()

	want := `SELECT "accounts".* FROM "accounts" WHERE "id" = $1 FOR UPDATE`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 1 {
		t.Errorf("args = %v", args)
	}
}

func TestBuildSelectForShare(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "accounts",
		ForShare(),
	)
	sql, _ := q.BuildSelect()

	want := `SELECT "accounts".* FROM "accounts" FOR SHARE`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
}

func TestBuildSelectForUpdateNowait(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "accounts",
		Where(`"id" = ?`, 1),
		ForUpdate(),
		Nowait(),
	)
	sql, _ := q.BuildSelect()

	want := `SELECT "accounts".* FROM "accounts" WHERE "id" = $1 FOR UPDATE NOWAIT`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
}

func TestBuildSelectForUpdateSkipLocked(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "jobs",
		ForUpdate(),
		SkipLocked(),
		Limit(1),
	)
	sql, _ := q.BuildSelect()

	want := `SELECT "jobs".* FROM "jobs" LIMIT $1 FOR UPDATE SKIP LOCKED`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
}

func TestBuildSelectForNoKeyUpdate(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "users", ForNoKeyUpdate())
	sql, _ := q.BuildSelect()

	want := `SELECT "users".* FROM "users" FOR NO KEY UPDATE`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
}

func TestBuildSelectForKeyShare(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "users", ForKeyShare())
	sql, _ := q.BuildSelect()

	want := `SELECT "users".* FROM "users" FOR KEY SHARE`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
}

func TestBuildSelectCTEWithLocking(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "accounts",
		WithCTE("locked", "SELECT id FROM accounts WHERE balance > ?", 1000),
		Where(`"id" IN (SELECT id FROM locked)`),
		ForUpdate(),
		Nowait(),
	)
	sql, args := q.BuildSelect()

	want := `WITH "locked" AS (SELECT id FROM accounts WHERE balance > $1) SELECT "accounts".* FROM "accounts" WHERE "id" IN (SELECT id FROM locked) FOR UPDATE NOWAIT`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 1 || args[0] != 1000 {
		t.Errorf("args = %v", args)
	}
}

func TestBuildBatchInsert(t *testing.T) {
	d := PostgresDialect{}
	sql, args := BuildBatchInsert(d, "users",
		[]string{"name", "email"},
		[][]any{
			{"Alice", "alice@example.com"},
			{"Bob", "bob@example.com"},
		},
		[]string{"id", "name", "email"},
	)

	want := `INSERT INTO "users" ("name", "email") VALUES ($1, $2), ($3, $4) RETURNING "id", "name", "email"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 4 || args[0] != "Alice" || args[1] != "alice@example.com" || args[2] != "Bob" || args[3] != "bob@example.com" {
		t.Errorf("args = %v", args)
	}
}

func TestBuildBatchInsertSingleRow(t *testing.T) {
	d := PostgresDialect{}
	sql, args := BuildBatchInsert(d, "users",
		[]string{"name"},
		[][]any{{"Alice"}},
		[]string{"id"},
	)

	want := `INSERT INTO "users" ("name") VALUES ($1) RETURNING "id"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 1 || args[0] != "Alice" {
		t.Errorf("args = %v", args)
	}
}

func TestBuildBatchInsertThreeRows(t *testing.T) {
	d := PostgresDialect{}
	sql, args := BuildBatchInsert(d, "tags",
		[]string{"name"},
		[][]any{{"go"}, {"rust"}, {"zig"}},
		nil,
	)

	want := `INSERT INTO "tags" ("name") VALUES ($1), ($2), ($3)`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 3 {
		t.Errorf("args = %v", args)
	}
}

func TestBuildBatchInsertEmpty(t *testing.T) {
	d := PostgresDialect{}
	sql, args := BuildBatchInsert(d, "users",
		[]string{"name"},
		nil,
		[]string{"id"},
	)

	if sql != "" {
		t.Errorf("expected empty sql, got: %s", sql)
	}
	if args != nil {
		t.Errorf("expected nil args, got: %v", args)
	}
}

func TestBuildBatchInsertNoReturning(t *testing.T) {
	d := testDialectNoReturning{}
	sql, _ := BuildBatchInsert(d, "users",
		[]string{"name", "email"},
		[][]any{{"Alice", "a@b.com"}, {"Bob", "b@b.com"}},
		[]string{"id"},
	)

	want := `INSERT INTO "users" ("name", "email") VALUES (?, ?), (?, ?)`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
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

// --- Subquery tests ---

func TestWhereSubqueryIn(t *testing.T) {
	d := PostgresDialect{}
	sub := NewQuery(d, "orders",
		Select(`"user_id"`),
		Where(`"total" > ?`, 100),
	)
	q := NewQuery(d, "users",
		WhereSubquery(`"users"."id"`, "IN", sub),
	)
	sql, args := q.BuildSelect()

	want := `SELECT "users".* FROM "users" WHERE "users"."id" IN (SELECT "user_id" FROM "orders" WHERE "total" > $1)`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 1 || args[0] != 100 {
		t.Errorf("args = %v, want [100]", args)
	}
}

func TestWhereSubqueryNotIn(t *testing.T) {
	d := PostgresDialect{}
	sub := NewQuery(d, "blocked_users", Select(`"id"`))
	q := NewQuery(d, "users",
		WhereSubquery(`"users"."id"`, "NOT IN", sub),
	)
	sql, args := q.BuildSelect()

	want := `SELECT "users".* FROM "users" WHERE "users"."id" NOT IN (SELECT "id" FROM "blocked_users")`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 0 {
		t.Errorf("args = %v, want empty", args)
	}
}

func TestWhereSubqueryWithOuterArgs(t *testing.T) {
	d := PostgresDialect{}
	sub := NewQuery(d, "orders",
		Select(`"user_id"`),
		Where(`"status" = ?`, "active"),
	)
	q := NewQuery(d, "users",
		Where(`"age" > ?`, 18),
		WhereSubquery(`"users"."id"`, "IN", sub),
		Where(`"verified" = ?`, true),
	)
	sql, args := q.BuildSelect()

	want := `SELECT "users".* FROM "users" WHERE "age" > $1 AND "users"."id" IN (SELECT "user_id" FROM "orders" WHERE "status" = $2) AND "verified" = $3`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 3 {
		t.Errorf("args count = %d, want 3", len(args))
	}
	if args[0] != 18 || args[1] != "active" || args[2] != true {
		t.Errorf("args = %v", args)
	}
}

func TestWhereExists(t *testing.T) {
	d := PostgresDialect{}
	sub := NewQuery(d, "orders",
		Select("1"),
		Where(`"orders"."user_id" = "users"."id"`),
		Where(`"orders"."total" > ?`, 50),
	)
	q := NewQuery(d, "users",
		WhereExists(sub),
	)
	sql, args := q.BuildSelect()

	want := `SELECT "users".* FROM "users" WHERE EXISTS (SELECT 1 FROM "orders" WHERE "orders"."user_id" = "users"."id" AND "orders"."total" > $1)`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 1 || args[0] != 50 {
		t.Errorf("args = %v, want [50]", args)
	}
}

func TestWhereNotExists(t *testing.T) {
	d := PostgresDialect{}
	sub := NewQuery(d, "bans", Select("1"), Where(`"bans"."user_id" = "users"."id"`))
	q := NewQuery(d, "users", WhereNotExists(sub))
	sql, args := q.BuildSelect()

	want := `SELECT "users".* FROM "users" WHERE NOT EXISTS (SELECT 1 FROM "bans" WHERE "bans"."user_id" = "users"."id")`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 0 {
		t.Errorf("args = %v, want empty", args)
	}
}

func TestWhereScalarSubquery(t *testing.T) {
	d := PostgresDialect{}
	sub := NewQuery(d, "orders",
		Select("MAX(total)"),
		Where(`"user_id" = ?`, 42),
	)
	q := NewQuery(d, "payments",
		Where(`"amount" > ?`, 0),
		WhereSubquery(`"amount"`, "=", sub),
	)
	sql, args := q.BuildSelect()

	want := `SELECT "payments".* FROM "payments" WHERE "amount" > $1 AND "amount" = (SELECT MAX(total) FROM "orders" WHERE "user_id" = $2)`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 2 || args[0] != 0 || args[1] != 42 {
		t.Errorf("args = %v", args)
	}
}

// --- FROM subquery tests ---

func TestFromSubquery(t *testing.T) {
	d := PostgresDialect{}
	inner := NewQuery(d, "orders",
		Select(`"user_id"`, `SUM("total") AS "order_total"`),
		GroupBy(`"user_id"`),
		Having(`SUM("total") > ?`, 1000),
	)
	q := NewQuery(d, "",
		FromSubquery("t", inner),
		Select(`t."user_id"`, `t."order_total"`),
		OrderBy(`t."order_total" DESC`),
	)
	sql, args := q.BuildSelect()

	want := `SELECT t."user_id", t."order_total" FROM (SELECT "user_id", SUM("total") AS "order_total" FROM "orders" GROUP BY "user_id" HAVING SUM("total") > $1) AS t ORDER BY t."order_total" DESC`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 1 || args[0] != 1000 {
		t.Errorf("args = %v, want [1000]", args)
	}
}

func TestFromSubqueryDefaultCols(t *testing.T) {
	d := PostgresDialect{}
	inner := NewQuery(d, "users", Where(`"active" = ?`, true))
	q := NewQuery(d, "", FromSubquery("active_users", inner))
	sql, args := q.BuildSelect()

	want := `SELECT active_users.* FROM (SELECT "users".* FROM "users" WHERE "active" = $1) AS active_users`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 1 || args[0] != true {
		t.Errorf("args = %v", args)
	}
}

func TestFromSubqueryWithOuterWhere(t *testing.T) {
	d := PostgresDialect{}
	inner := NewQuery(d, "orders",
		Select(`"user_id"`, `COUNT(*) AS "cnt"`),
		GroupBy(`"user_id"`),
	)
	q := NewQuery(d, "",
		FromSubquery("t", inner),
		Select(`t."user_id"`),
		Where(`t."cnt" > ?`, 5),
	)
	sql, args := q.BuildSelect()

	want := `SELECT t."user_id" FROM (SELECT "user_id", COUNT(*) AS "cnt" FROM "orders" GROUP BY "user_id") AS t WHERE t."cnt" > $1`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 1 || args[0] != 5 {
		t.Errorf("args = %v, want [5]", args)
	}
}

// --- UNION / INTERSECT / EXCEPT tests ---

func TestUnion(t *testing.T) {
	d := PostgresDialect{}
	q1 := NewQuery(d, "customers", Select(`"name"`), Where(`"type" = ?`, "premium"))
	q2 := NewQuery(d, "vendors", Select(`"name"`), Where(`"active" = ?`, true))
	q1Mod := NewQuery(d, "customers",
		Select(`"name"`),
		Where(`"type" = ?`, "premium"),
		Union(q2),
	)
	sql, args := q1Mod.BuildSelect()

	want := `SELECT "name" FROM "customers" WHERE "type" = $1 UNION SELECT "name" FROM "vendors" WHERE "active" = $2`
	_ = q1
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 2 || args[0] != "premium" || args[1] != true {
		t.Errorf("args = %v", args)
	}
}

func TestUnionAll(t *testing.T) {
	d := PostgresDialect{}
	q2 := NewQuery(d, "archived_orders", Select(`"id"`, `"total"`))
	q := NewQuery(d, "orders",
		Select(`"id"`, `"total"`),
		UnionAll(q2),
	)
	sql, args := q.BuildSelect()

	want := `SELECT "id", "total" FROM "orders" UNION ALL SELECT "id", "total" FROM "archived_orders"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 0 {
		t.Errorf("args = %v, want empty", args)
	}
}

func TestIntersect(t *testing.T) {
	d := PostgresDialect{}
	q2 := NewQuery(d, "premium_users", Select(`"id"`))
	q := NewQuery(d, "active_users",
		Select(`"id"`),
		Intersect(q2),
	)
	sql, args := q.BuildSelect()

	want := `SELECT "id" FROM "active_users" INTERSECT SELECT "id" FROM "premium_users"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 0 {
		t.Errorf("args = %v, want empty", args)
	}
}

func TestExcept(t *testing.T) {
	d := PostgresDialect{}
	q2 := NewQuery(d, "blocked_users", Select(`"id"`))
	q := NewQuery(d, "users",
		Select(`"id"`),
		Except(q2),
	)
	sql, args := q.BuildSelect()

	want := `SELECT "id" FROM "users" EXCEPT SELECT "id" FROM "blocked_users"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 0 {
		t.Errorf("args = %v, want empty", args)
	}
}

func TestMultipleUnions(t *testing.T) {
	d := PostgresDialect{}
	q2 := NewQuery(d, "vendors", Select(`"name"`))
	q3 := NewQuery(d, "partners", Select(`"name"`))
	q := NewQuery(d, "customers",
		Select(`"name"`),
		Union(q2),
		Union(q3),
	)
	sql, args := q.BuildSelect()

	want := `SELECT "name" FROM "customers" UNION SELECT "name" FROM "vendors" UNION SELECT "name" FROM "partners"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 0 {
		t.Errorf("args = %v, want empty", args)
	}
}

func TestUnionWithArgs(t *testing.T) {
	d := PostgresDialect{}
	q2 := NewQuery(d, "orders",
		Select(`"user_id"`),
		Where(`"total" > ?`, 200),
	)
	q := NewQuery(d, "users",
		Select(`"id"`),
		Where(`"active" = ?`, true),
		UnionAll(q2),
	)
	sql, args := q.BuildSelect()

	want := `SELECT "id" FROM "users" WHERE "active" = $1 UNION ALL SELECT "user_id" FROM "orders" WHERE "total" > $2`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 2 || args[0] != true || args[1] != 200 {
		t.Errorf("args = %v", args)
	}
}

func TestIntersectAll(t *testing.T) {
	d := PostgresDialect{}
	q2 := NewQuery(d, "b", Select(`"id"`))
	q := NewQuery(d, "a", Select(`"id"`), IntersectAll(q2))
	sql, _ := q.BuildSelect()

	want := `SELECT "id" FROM "a" INTERSECT ALL SELECT "id" FROM "b"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
}

func TestExceptAll(t *testing.T) {
	d := PostgresDialect{}
	q2 := NewQuery(d, "b", Select(`"id"`))
	q := NewQuery(d, "a", Select(`"id"`), ExceptAll(q2))
	sql, _ := q.BuildSelect()

	want := `SELECT "id" FROM "a" EXCEPT ALL SELECT "id" FROM "b"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
}

// --- Window function tests ---

func TestWindowFunctionRowNumber(t *testing.T) {
	d := PostgresDialect{}
	w := NewWindowDef().OrderBy(`"created_at" DESC`)
	q := NewQuery(d, "posts",
		Select(`"id"`, `"title"`),
		SelectWithWindow("ROW_NUMBER()", w, "row_num"),
	)
	sql, args := q.BuildSelect()

	want := `SELECT "id", "title", ROW_NUMBER() OVER (ORDER BY "created_at" DESC) AS row_num FROM "posts"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 0 {
		t.Errorf("args = %v, want empty", args)
	}
}

func TestWindowFunctionPartitionBy(t *testing.T) {
	d := PostgresDialect{}
	w := NewWindowDef().PartitionBy(`"department"`).OrderBy(`"salary" DESC`)
	q := NewQuery(d, "employees",
		Select(`"name"`, `"department"`, `"salary"`),
		SelectWithWindow("RANK()", w, "salary_rank"),
	)
	sql, args := q.BuildSelect()

	want := `SELECT "name", "department", "salary", RANK() OVER (PARTITION BY "department" ORDER BY "salary" DESC) AS salary_rank FROM "employees"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 0 {
		t.Errorf("args = %v, want empty", args)
	}
}

func TestWindowFunctionWithFrame(t *testing.T) {
	d := PostgresDialect{}
	w := NewWindowDef().
		PartitionBy(`"user_id"`).
		OrderBy(`"created_at"`).
		Frame("ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW")
	q := NewQuery(d, "orders",
		Select(`"user_id"`, `"total"`),
		SelectWithWindow("SUM(total)", w, "running_total"),
	)
	sql, _ := q.BuildSelect()

	want := `SELECT "user_id", "total", SUM(total) OVER (PARTITION BY "user_id" ORDER BY "created_at" ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS running_total FROM "orders"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
}

func TestWindowFunctionNoAlias(t *testing.T) {
	d := PostgresDialect{}
	w := NewWindowDef().OrderBy(`"id"`)
	q := NewQuery(d, "items",
		Select(`"id"`),
		SelectWithWindow("LAG(price, 1)", w, ""),
	)
	sql, _ := q.BuildSelect()

	want := `SELECT "id", LAG(price, 1) OVER (ORDER BY "id") FROM "items"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
}

func TestWindowFunctionDenseRank(t *testing.T) {
	d := PostgresDialect{}
	w := NewWindowDef().PartitionBy(`"category"`).OrderBy(`"score" DESC`)
	q := NewQuery(d, "products",
		Select(`"name"`, `"category"`, `"score"`),
		SelectWithWindow("DENSE_RANK()", w, "rank"),
	)
	sql, _ := q.BuildSelect()

	want := `SELECT "name", "category", "score", DENSE_RANK() OVER (PARTITION BY "category" ORDER BY "score" DESC) AS rank FROM "products"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
}

// --- INSERT FROM SELECT tests ---

func TestBuildInsertSelect(t *testing.T) {
	d := PostgresDialect{}
	selQ := NewQuery(d, "temp_users",
		Select(`"name"`, `"email"`),
		Where(`"verified" = ?`, true),
	)
	sql, args := BuildInsertSelect(d, "users", []string{"name", "email"}, selQ, []string{"id"})

	want := `INSERT INTO "users" ("name", "email") SELECT "name", "email" FROM "temp_users" WHERE "verified" = $1 RETURNING "id"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 1 || args[0] != true {
		t.Errorf("args = %v, want [true]", args)
	}
}

func TestBuildInsertSelectNoReturning(t *testing.T) {
	d := PostgresDialect{}
	selQ := NewQuery(d, "staging", Select(`"col_a"`, `"col_b"`))
	sql, args := BuildInsertSelect(d, "target", []string{"col_a", "col_b"}, selQ, nil)

	want := `INSERT INTO "target" ("col_a", "col_b") SELECT "col_a", "col_b" FROM "staging"`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 0 {
		t.Errorf("args = %v, want empty", args)
	}
}

func TestBuildInsertSelectWithLimit(t *testing.T) {
	d := PostgresDialect{}
	selQ := NewQuery(d, "events",
		Select(`"user_id"`, `"event_type"`),
		Where(`"created_at" > ?`, "2024-01-01"),
		Limit(100),
	)
	sql, args := BuildInsertSelect(d, "audit_log", []string{"user_id", "event_type"}, selQ, nil)

	want := `INSERT INTO "audit_log" ("user_id", "event_type") SELECT "user_id", "event_type" FROM "events" WHERE "created_at" > $1 LIMIT $2`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 2 || args[0] != "2024-01-01" || args[1] != 100 {
		t.Errorf("args = %v", args)
	}
}

// --- Nested subquery tests ---

func TestNestedSubqueries(t *testing.T) {
	d := PostgresDialect{}
	// SELECT * FROM users WHERE id IN (SELECT user_id FROM orders WHERE product_id IN (SELECT id FROM products WHERE price > $1))
	innermost := NewQuery(d, "products",
		Select(`"id"`),
		Where(`"price" > ?`, 99),
	)
	middle := NewQuery(d, "orders",
		Select(`"user_id"`),
		WhereSubquery(`"product_id"`, "IN", innermost),
	)
	q := NewQuery(d, "users",
		WhereSubquery(`"id"`, "IN", middle),
	)
	sql, args := q.BuildSelect()

	want := `SELECT "users".* FROM "users" WHERE "id" IN (SELECT "user_id" FROM "orders" WHERE "product_id" IN (SELECT "id" FROM "products" WHERE "price" > $1))`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 1 || args[0] != 99 {
		t.Errorf("args = %v, want [99]", args)
	}
}

func TestSubqueryInFromWithSubqueryInWhere(t *testing.T) {
	d := PostgresDialect{}
	// FROM (SELECT ...) AS t WHERE t.id IN (SELECT ...)
	inner := NewQuery(d, "orders",
		Select(`"user_id"`, `SUM("total") AS "sum_total"`),
		Where(`"status" = ?`, "complete"),
		GroupBy(`"user_id"`),
	)
	filterSub := NewQuery(d, "premium_users", Select(`"id"`))
	q := NewQuery(d, "",
		FromSubquery("t", inner),
		Select(`t."user_id"`, `t."sum_total"`),
		WhereSubquery(`t."user_id"`, "IN", filterSub),
	)
	sql, args := q.BuildSelect()

	want := `SELECT t."user_id", t."sum_total" FROM (SELECT "user_id", SUM("total") AS "sum_total" FROM "orders" WHERE "status" = $1 GROUP BY "user_id") AS t WHERE t."user_id" IN (SELECT "id" FROM "premium_users")`
	if sql != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", sql, want)
	}
	if len(args) != 1 || args[0] != "complete" {
		t.Errorf("args = %v", args)
	}
}

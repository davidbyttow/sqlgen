package runtime

import "testing"

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
		LeftJoin(`"users" ON "users"."id" = "posts"."author_id"`, ""),
	)
	sql, _ := q.BuildSelect()

	want := `SELECT "posts".* FROM "posts" LEFT JOIN "users" ON "users"."id" = "posts"."author_id" ON `
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

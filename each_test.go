package sqlgen

import (
	"context"
	"database/sql"
	"errors"
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

// --- mock infrastructure for Each/Cursor tests ---

// mockRow implements the scanner interface passed to ScanRow.
type mockRow struct {
	vals []any
	err  error
}

func (m *mockRow) Scan(dest ...any) error {
	if m.err != nil {
		return m.err
	}
	for i, d := range dest {
		if i < len(m.vals) {
			*(d.(*string)) = m.vals[i].(string)
		}
	}
	return nil
}

// mockItem is a simple RowScanner for testing.
type mockItem struct {
	Name string
}

func (m *mockItem) ScanRow(scanner interface{ Scan(dest ...any) error }) error {
	return scanner.Scan(&m.Name)
}

// mockRows simulates *sql.Rows for Each/Cursor tests.
type mockRows struct {
	data    [][]any // each inner slice is one row
	scanErr error   // if set, Scan returns this
	pos     int
	closed  bool
}

func (m *mockRows) Next() bool {
	if m.pos < len(m.data) {
		return true
	}
	return false
}

func (m *mockRows) Scan(dest ...any) error {
	if m.scanErr != nil {
		return m.scanErr
	}
	row := m.data[m.pos]
	for i, d := range dest {
		if i < len(row) {
			*(d.(*string)) = row[i].(string)
		}
	}
	m.pos++
	return nil
}

func (m *mockRows) Close() error {
	m.closed = true
	return nil
}

func (m *mockRows) Err() error {
	return nil
}

// testExec satisfies Executor with configurable behavior for testing.
type testExec struct {
	rows    *mockRows
	queryFn func(query string, args []any) // optional hook to inspect SQL
	err     error
}

func (m *testExec) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	return nil, errors.New("not implemented")
}

func (m *testExec) QueryContext(_ context.Context, query string, args ...any) (*sql.Rows, error) {
	if m.queryFn != nil {
		m.queryFn(query, args)
	}
	if m.err != nil {
		return nil, m.err
	}
	return nil, errors.New("testExec: use query building tests instead")
}

func (m *testExec) QueryRowContext(_ context.Context, query string, args ...any) *sql.Row {
	return nil
}

// --- Query building tests for Each ---

func TestEachQueryBuilding(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "posts", Where(`"status" = ?`, "active"), OrderBy(`"created_at" DESC`), Limit(20))
	query, args := q.BuildSelect()

	want := `SELECT "posts".* FROM "posts" WHERE "status" = $1 ORDER BY "created_at" DESC LIMIT $2`
	if query != want {
		t.Errorf("query:\n  got  %q\n  want %q", query, want)
	}
	if len(args) != 2 {
		t.Fatalf("args len = %d, want 2", len(args))
	}
	if args[0] != "active" {
		t.Errorf("args[0] = %v, want 'active'", args[0])
	}
	if args[1] != 20 {
		t.Errorf("args[1] = %v, want 20", args[1])
	}
}

func TestEachQueryBuildingWithMultipleWhere(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "users",
		Where(`"age" > ?`, 18),
		Where(`"active" = ?`, true),
		Limit(5),
	)
	query, args := q.BuildSelect()

	want := `SELECT "users".* FROM "users" WHERE "age" > $1 AND "active" = $2 LIMIT $3`
	if query != want {
		t.Errorf("query:\n  got  %q\n  want %q", query, want)
	}
	if len(args) != 3 {
		t.Fatalf("args len = %d, want 3", len(args))
	}
}

func TestEachQueryBuildingNoMods(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "items")
	query, args := q.BuildSelect()

	want := `SELECT "items".* FROM "items"`
	if query != want {
		t.Errorf("query = %q, want %q", query, want)
	}
	if len(args) != 0 {
		t.Errorf("args = %v, want empty", args)
	}
}

// --- Query building tests for Cursor ---

func TestCursorQueryBuilding(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "events",
		Where(`"type" = ?`, "click"),
		OrderBy(`"timestamp" DESC`),
		Limit(100),
		Offset(50),
	)
	query, args := q.BuildSelect()

	want := `SELECT "events".* FROM "events" WHERE "type" = $1 ORDER BY "timestamp" DESC LIMIT $2 OFFSET $3`
	if query != want {
		t.Errorf("query:\n  got  %q\n  want %q", query, want)
	}
	if len(args) != 3 {
		t.Fatalf("args len = %d, want 3", len(args))
	}
	if args[0] != "click" {
		t.Errorf("args[0] = %v", args[0])
	}
	if args[1] != 100 {
		t.Errorf("args[1] = %v", args[1])
	}
	if args[2] != 50 {
		t.Errorf("args[2] = %v", args[2])
	}
}

func TestEachExecutorError(t *testing.T) {
	// When QueryContext returns an error, Each should propagate it.
	d := PostgresDialect{}
	q := NewQuery(d, "users")

	wantErr := errors.New("connection refused")
	exec := &testExec{err: wantErr}

	err := Each(context.Background(), exec, q, func() *mockItem { return &mockItem{} }, func(item *mockItem) error {
		return nil
	})
	// mockExecutor returns its own error ("mockExecutor: use query building tests instead")
	// unless we set exec.err, but Each calls exec.QueryContext which returns *sql.Rows.
	// Since our mock can't return a real *sql.Rows, we check that the error is propagated.
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		// The mock returns the configured error.
		t.Errorf("err = %v, want %v", err, wantErr)
	}
}

func TestNewCursorExecutorError(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "users")

	wantErr := errors.New("db down")
	exec := &testExec{err: wantErr}

	cursor, err := NewCursor(context.Background(), exec, q, func() *mockItem { return &mockItem{} })
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
	if cursor != nil {
		t.Error("cursor should be nil on error")
	}
}

func TestEachQueryWithSelectCols(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "users", Select(`"id"`, `"name"`), Limit(10))
	query, args := q.BuildSelect()

	want := `SELECT "id", "name" FROM "users" LIMIT $1`
	if query != want {
		t.Errorf("query = %q, want %q", query, want)
	}
	if len(args) != 1 || args[0] != 10 {
		t.Errorf("args = %v", args)
	}
}

func TestEachQueryWithJoin(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "posts",
		Join(`"users"`, `"users"."id" = "posts"."user_id"`),
		Where(`"users"."active" = ?`, true),
	)
	query, args := q.BuildSelect()

	want := `SELECT "posts".* FROM "posts" JOIN "users" ON "users"."id" = "posts"."user_id" WHERE "users"."active" = $1`
	if query != want {
		t.Errorf("query:\n  got  %q\n  want %q", query, want)
	}
	if len(args) != 1 {
		t.Errorf("args = %v", args)
	}
}

func TestEachQueryInspection(t *testing.T) {
	// Verify that the SQL sent to the executor matches expectations.
	d := PostgresDialect{}
	q := NewQuery(d, "logs", Where(`"level" = ?`, "error"), Limit(50))

	var capturedQuery string
	var capturedArgs []any
	exec := &testExec{
		err: errors.New("stop"),
		queryFn: func(query string, args []any) {
			capturedQuery = query
			capturedArgs = args
		},
	}

	_ = Each(context.Background(), exec, q, func() *mockItem { return &mockItem{} }, func(item *mockItem) error {
		return nil
	})

	wantQuery := `SELECT "logs".* FROM "logs" WHERE "level" = $1 LIMIT $2`
	if capturedQuery != wantQuery {
		t.Errorf("captured query:\n  got  %q\n  want %q", capturedQuery, wantQuery)
	}
	if len(capturedArgs) != 2 {
		t.Fatalf("captured args len = %d, want 2", len(capturedArgs))
	}
	if capturedArgs[0] != "error" {
		t.Errorf("capturedArgs[0] = %v", capturedArgs[0])
	}
	if capturedArgs[1] != 50 {
		t.Errorf("capturedArgs[1] = %v", capturedArgs[1])
	}
}

func TestNewCursorQueryInspection(t *testing.T) {
	d := PostgresDialect{}
	q := NewQuery(d, "events", Where(`"kind" = ?`, "pageview"), OrderBy(`"ts"`))

	var capturedQuery string
	exec := &testExec{
		err: errors.New("stop"),
		queryFn: func(query string, args []any) {
			capturedQuery = query
		},
	}

	_, _ = NewCursor(context.Background(), exec, q, func() *mockItem { return &mockItem{} })

	want := `SELECT "events".* FROM "events" WHERE "kind" = $1 ORDER BY "ts"`
	if capturedQuery != want {
		t.Errorf("captured query:\n  got  %q\n  want %q", capturedQuery, want)
	}
}

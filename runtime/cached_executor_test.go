package runtime

import (
	"context"
	"database/sql"
	"sync"
	"sync/atomic"
	"testing"
)

// Compile-time check that CachedExecutor implements Executor.
var _ Executor = (*CachedExecutor)(nil)

func TestCachedExecutorCloseEmpty(t *testing.T) {
	c := &CachedExecutor{stmts: make(map[string]*sql.Stmt)}
	if err := c.Close(); err != nil {
		t.Errorf("Close empty cache: %v", err)
	}
}

func TestCachedExecutorLen(t *testing.T) {
	c := &CachedExecutor{stmts: make(map[string]*sql.Stmt)}
	if c.Len() != 0 {
		t.Errorf("Len = %d, want 0", c.Len())
	}
}

func TestCachedExecutorConcurrentLen(t *testing.T) {
	c := &CachedExecutor{stmts: make(map[string]*sql.Stmt)}
	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.Len()
		}()
	}
	wg.Wait()
}

// mockPreparableExecutor tracks prepare calls for verifying caching behavior.
type mockPreparableExecutor struct {
	prepareCount atomic.Int64
}

func (m *mockPreparableExecutor) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return nil, nil
}

func (m *mockPreparableExecutor) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return nil, nil
}

func (m *mockPreparableExecutor) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return nil
}

func (m *mockPreparableExecutor) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	m.prepareCount.Add(1)
	// We can't easily create a real *sql.Stmt without a DB, but we can verify
	// the prepare count. Returning nil will cause a panic on use, but we test
	// the caching logic separately.
	return nil, nil
}

func TestCachedExecutorPreparesOnce(t *testing.T) {
	mock := &mockPreparableExecutor{}
	c := NewCachedExecutor(mock)

	ctx := context.Background()
	query := "SELECT 1"

	// First call should prepare.
	_, _ = c.getOrPrepare(ctx, query)
	if mock.prepareCount.Load() != 1 {
		t.Fatalf("prepare count = %d, want 1", mock.prepareCount.Load())
	}

	// Second call should hit cache.
	_, _ = c.getOrPrepare(ctx, query)
	if mock.prepareCount.Load() != 1 {
		t.Fatalf("prepare count = %d after cache hit, want 1", mock.prepareCount.Load())
	}

	// Different query should prepare again.
	_, _ = c.getOrPrepare(ctx, "SELECT 2")
	if mock.prepareCount.Load() != 2 {
		t.Fatalf("prepare count = %d, want 2", mock.prepareCount.Load())
	}

	if c.Len() != 2 {
		t.Fatalf("Len = %d, want 2", c.Len())
	}
}

func TestCachedExecutorConcurrentPrepare(t *testing.T) {
	mock := &mockPreparableExecutor{}
	c := NewCachedExecutor(mock)

	ctx := context.Background()
	query := "SELECT 1"

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = c.getOrPrepare(ctx, query)
		}()
	}
	wg.Wait()

	// Should have prepared exactly once despite concurrent access.
	if mock.prepareCount.Load() != 1 {
		t.Errorf("prepare count = %d, want 1 (concurrent)", mock.prepareCount.Load())
	}
}

package runtime

import (
	"context"
	"database/sql"
	"sync"
)

// Preparer can prepare SQL statements. Satisfied by *sql.DB and *sql.Tx.
type Preparer interface {
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}

// CachedExecutor wraps a database connection with automatic prepared statement
// caching. The same query string is prepared once and reused on subsequent calls.
//
// Safe for concurrent use. Create one per *sql.DB for long-lived caching,
// or one per *sql.Tx for transaction-scoped caching.
//
// Usage:
//
//	cached := runtime.NewCachedExecutor(db)
//	defer cached.Close()
//	user, err := models.FindUserByPK(ctx, cached, 1)
type CachedExecutor struct {
	exec  Executor
	prep  Preparer
	mu    sync.RWMutex
	stmts map[string]*sql.Stmt
}

// NewCachedExecutor creates a CachedExecutor. The conn must implement both
// Executor and Preparer (satisfied by *sql.DB and *sql.Tx).
func NewCachedExecutor(conn interface {
	Executor
	Preparer
}) *CachedExecutor {
	return &CachedExecutor{
		exec:  conn,
		prep:  conn,
		stmts: make(map[string]*sql.Stmt),
	}
}

func (c *CachedExecutor) getOrPrepare(ctx context.Context, query string) (*sql.Stmt, error) {
	c.mu.RLock()
	if stmt, ok := c.stmts[query]; ok {
		c.mu.RUnlock()
		return stmt, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock.
	if stmt, ok := c.stmts[query]; ok {
		return stmt, nil
	}

	stmt, err := c.prep.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	c.stmts[query] = stmt
	return stmt, nil
}

// ExecContext implements Executor.
func (c *CachedExecutor) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	stmt, err := c.getOrPrepare(ctx, query)
	if err != nil {
		return nil, err
	}
	return stmt.ExecContext(ctx, args...)
}

// QueryContext implements Executor.
func (c *CachedExecutor) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	stmt, err := c.getOrPrepare(ctx, query)
	if err != nil {
		return nil, err
	}
	return stmt.QueryContext(ctx, args...)
}

// QueryRowContext implements Executor.
func (c *CachedExecutor) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	stmt, err := c.getOrPrepare(ctx, query)
	if err != nil {
		// Fall back to direct execution so the error surfaces on Scan.
		return c.exec.QueryRowContext(ctx, query, args...)
	}
	return stmt.QueryRowContext(ctx, args...)
}

// Close closes all cached prepared statements. Always call when done.
func (c *CachedExecutor) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var firstErr error
	for query, stmt := range c.stmts {
		if err := stmt.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(c.stmts, query)
	}
	return firstErr
}

// Len returns the number of cached prepared statements.
func (c *CachedExecutor) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.stmts)
}

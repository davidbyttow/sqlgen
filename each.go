package sqlgen

import (
	"context"
	"database/sql"
)

// RowScanner is the interface for types that can scan from a database row.
type RowScanner interface {
	ScanRow(scanner interface{ Scan(dest ...any) error }) error
}

// Each executes a query and calls fn for each row. Rows are scanned into
// new instances created by the newFn factory. Iteration stops early if fn
// returns an error.
func Each[T RowScanner](ctx context.Context, exec Executor, q *Query, newFn func() T, fn func(T) error) error {
	query, args := q.BuildSelect()
	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		o := newFn()
		if err := o.ScanRow(rows); err != nil {
			return err
		}
		if err := fn(o); err != nil {
			return err
		}
	}
	return rows.Err()
}

// Cursor wraps *sql.Rows and provides typed iteration over query results.
type Cursor[T RowScanner] struct {
	rows  *sql.Rows
	newFn func() T
	err   error
}

// NewCursor creates a Cursor that iterates over the given query results.
func NewCursor[T RowScanner](ctx context.Context, exec Executor, q *Query, newFn func() T) (*Cursor[T], error) {
	query, args := q.BuildSelect()
	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &Cursor[T]{rows: rows, newFn: newFn}, nil
}

// Next advances the cursor to the next row and scans it. Returns false
// when there are no more rows or an error occurred (check Err()).
func (c *Cursor[T]) Next() (T, bool) {
	if !c.rows.Next() {
		return c.newFn(), false
	}
	o := c.newFn()
	if err := o.ScanRow(c.rows); err != nil {
		c.err = err
		return o, false
	}
	return o, true
}

// Err returns any error encountered during iteration.
func (c *Cursor[T]) Err() error {
	if c.err != nil {
		return c.err
	}
	return c.rows.Err()
}

// Close closes the underlying rows.
func (c *Cursor[T]) Close() error {
	return c.rows.Close()
}

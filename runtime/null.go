// Package runtime provides the minimal runtime library that generated code imports.
// Keep this package small and stable; it's a public API.
package runtime

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// Null represents an optional value that may be NULL in the database.
// The zero value is null (Valid = false).
type Null[T any] struct {
	Val   T
	Valid bool // Valid is true when Val is set (not NULL).
}

// NewNull creates a non-null Null[T] with the given value.
func NewNull[T any](v T) Null[T] {
	return Null[T]{Val: v, Valid: true}
}

// NullVal returns a null Null[T].
func NullVal[T any]() Null[T] {
	return Null[T]{}
}

// Set sets the value and marks it as valid.
func (n *Null[T]) Set(v T) {
	n.Val = v
	n.Valid = true
}

// Clear resets to null.
func (n *Null[T]) Clear() {
	var zero T
	n.Val = zero
	n.Valid = false
}

// Ptr returns a pointer to the value, or nil if null.
func (n Null[T]) Ptr() *T {
	if !n.Valid {
		return nil
	}
	return &n.Val
}

// FromPtr creates a Null[T] from a pointer. Nil pointer means null.
func FromPtr[T any](p *T) Null[T] {
	if p == nil {
		return Null[T]{}
	}
	return Null[T]{Val: *p, Valid: true}
}

// MarshalJSON implements json.Marshaler. Null values produce "null".
func (n Null[T]) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.Val)
}

// UnmarshalJSON implements json.Unmarshaler. JSON "null" produces a null value.
func (n *Null[T]) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Valid = false
		var zero T
		n.Val = zero
		return nil
	}
	if err := json.Unmarshal(data, &n.Val); err != nil {
		return err
	}
	n.Valid = true
	return nil
}

// Scan implements sql.Scanner for reading from the database.
func (n *Null[T]) Scan(src any) error {
	if src == nil {
		n.Valid = false
		var zero T
		n.Val = zero
		return nil
	}

	// If T itself implements sql.Scanner, delegate.
	if scanner, ok := any(&n.Val).(sql.Scanner); ok {
		if err := scanner.Scan(src); err != nil {
			return err
		}
		n.Valid = true
		return nil
	}

	// Direct type assertion.
	if v, ok := src.(T); ok {
		n.Val = v
		n.Valid = true
		return nil
	}

	// Try common conversions.
	return fmt.Errorf("sqlgen: cannot scan %T into Null[%T]", src, n.Val)
}

// Value implements driver.Valuer for writing to the database.
func (n Null[T]) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}

	// If T implements driver.Valuer, delegate.
	if valuer, ok := any(n.Val).(driver.Valuer); ok {
		return valuer.Value()
	}

	return n.Val, nil
}

// String returns a string representation for debugging.
func (n Null[T]) String() string {
	if !n.Valid {
		return "NULL"
	}
	return fmt.Sprintf("%v", n.Val)
}

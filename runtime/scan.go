package runtime

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

// ColumnMap maps column names to their index in a struct's fields.
// Generated code provides these statically per model.
type ColumnMap struct {
	Columns []string        // Column names in field order
	indices map[string]int  // Column name -> field index
}

// NewColumnMap creates a column mapping from ordered column names.
func NewColumnMap(cols ...string) *ColumnMap {
	m := &ColumnMap{
		Columns: cols,
		indices: make(map[string]int, len(cols)),
	}
	for i, c := range cols {
		m.indices[c] = i
	}
	return m
}

// Index returns the field index for a column name, or -1 if not found.
func (m *ColumnMap) Index(col string) int {
	idx, ok := m.indices[col]
	if !ok {
		return -1
	}
	return idx
}

// ScanRow scans a single row into a slice of destination pointers, ordered
// to match the columns returned by the query.
func ScanRow(row *sql.Row, dests ...any) error {
	return row.Scan(dests...)
}

// ScanRows scans a sql.Rows result set, calling scanFn for each row.
// scanFn receives the rows and should scan into the target struct.
func ScanRows[T any](rows *sql.Rows, scanFn func(*sql.Rows) (T, error)) ([]T, error) {
	defer rows.Close()

	var result []T
	for rows.Next() {
		item, err := scanFn(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// ScanOne scans a single row from sql.Rows, returning an error if no rows or more than one.
func ScanOne[T any](rows *sql.Rows, scanFn func(*sql.Rows) (T, error)) (T, error) {
	defer rows.Close()

	var zero T
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return zero, err
		}
		return zero, sql.ErrNoRows
	}

	item, err := scanFn(rows)
	if err != nil {
		return zero, err
	}

	return item, nil
}

// FieldPointers returns a slice of pointers to struct fields in column order.
// This is used by generated code to create scan destinations.
// The struct must have exported fields matching the column map.
func FieldPointers(v any, cols []string) ([]any, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("sqlgen: FieldPointers requires a struct, got %T", v)
	}

	rt := rv.Type()
	// Build a map of json/db tag -> field index for lookup.
	tagMap := make(map[string]int, rt.NumField())
	for i := range rt.NumField() {
		f := rt.Field(i)
		if tag, ok := f.Tag.Lookup("db"); ok {
			tagMap[tag] = i
		} else if tag, ok := f.Tag.Lookup("json"); ok {
			// Strip options like ",omitempty"
			if commaIdx := strings.Index(tag, ","); commaIdx >= 0 {
				tag = tag[:commaIdx]
			}
			if tag != "" && tag != "-" {
				tagMap[tag] = i
			}
		}
	}

	ptrs := make([]any, len(cols))
	for i, col := range cols {
		if fieldIdx, ok := tagMap[col]; ok {
			ptrs[i] = rv.Field(fieldIdx).Addr().Interface()
		} else {
			// Fallback: discard the value.
			var discard any
			ptrs[i] = &discard
		}
	}
	return ptrs, nil
}

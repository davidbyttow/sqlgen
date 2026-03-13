package sqlgen

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
)

// Bind executes a query and scans the results into dest.
// dest must be a pointer to a struct (for a single row) or a pointer to a slice of structs.
//
// Column-to-field matching uses `db` tags first, then `json` tags.
// Unmatched columns are silently discarded.
//
// Examples:
//
//	var users []User
//	err := sqlgen.Bind(ctx, db, q, &users)
//
//	var user User
//	err := sqlgen.Bind(ctx, db, q, &user) // returns sql.ErrNoRows if none
func Bind(ctx context.Context, exec Executor, q *Query, dest any) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("sqlgen: Bind requires a non-nil pointer, got %T", dest)
	}

	elem := rv.Elem()
	switch elem.Kind() {
	case reflect.Slice:
		return bindSlice(ctx, exec, q, rv)
	case reflect.Struct:
		return bindOne(ctx, exec, q, rv)
	default:
		return fmt.Errorf("sqlgen: Bind requires pointer to struct or slice of structs, got *%s", elem.Type())
	}
}

func bindSlice(ctx context.Context, exec Executor, q *Query, slicePtr reflect.Value) error {
	sliceVal := slicePtr.Elem()
	elemType := sliceVal.Type().Elem()
	isPtr := elemType.Kind() == reflect.Pointer
	if isPtr {
		elemType = elemType.Elem()
	}
	if elemType.Kind() != reflect.Struct {
		return fmt.Errorf("sqlgen: Bind slice element must be a struct or *struct, got %s", sliceVal.Type().Elem())
	}

	queryStr, args := q.BuildSelect()
	rows, err := exec.QueryContext(ctx, queryStr, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	for rows.Next() {
		newElem := reflect.New(elemType)
		ptrs, err := FieldPointers(newElem.Interface(), cols)
		if err != nil {
			return err
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		if isPtr {
			sliceVal = reflect.Append(sliceVal, newElem)
		} else {
			sliceVal = reflect.Append(sliceVal, newElem.Elem())
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	slicePtr.Elem().Set(sliceVal)
	return nil
}

func bindOne(ctx context.Context, exec Executor, q *Query, structPtr reflect.Value) error {
	queryStr, args := q.BuildSelect()
	rows, err := exec.QueryContext(ctx, queryStr, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}

	ptrs, err := FieldPointers(structPtr.Interface(), cols)
	if err != nil {
		return err
	}
	return rows.Scan(ptrs...)
}

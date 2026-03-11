package runtime

import (
	"errors"
	"reflect"
)

// Constraint is a named database constraint for error matching.
type Constraint string

// pgErrorFields extracts the SQLSTATE code and constraint name from a Postgres
// driver error using reflection. Works with both pgx and lib/pq without importing them.
func pgErrorFields(err error) (code, constraint string, ok bool) {
	for err != nil {
		rv := reflect.ValueOf(err)
		if rv.Kind() == reflect.Pointer {
			rv = rv.Elem()
		}
		if rv.Kind() == reflect.Struct {
			codeField := rv.FieldByName("Code")
			if codeField.IsValid() && codeField.Kind() == reflect.String {
				code = codeField.String()
				// pgx: ConstraintName field
				if cn := rv.FieldByName("ConstraintName"); cn.IsValid() && cn.Kind() == reflect.String {
					return code, cn.String(), true
				}
				// lib/pq: Constraint field
				if cn := rv.FieldByName("Constraint"); cn.IsValid() && cn.Kind() == reflect.String {
					return code, cn.String(), true
				}
			}
		}
		err = errors.Unwrap(err)
	}
	return "", "", false
}

// IsUniqueViolation checks if err is a unique constraint violation for the given constraint.
func IsUniqueViolation(err error, c Constraint) bool {
	code, name, ok := pgErrorFields(err)
	return ok && code == "23505" && name == string(c)
}

// IsForeignKeyViolation checks if err is a FK constraint violation for the given constraint.
func IsForeignKeyViolation(err error, c Constraint) bool {
	code, name, ok := pgErrorFields(err)
	return ok && code == "23503" && name == string(c)
}

// IsNotNullViolation checks if err is a NOT NULL violation for the given constraint.
func IsNotNullViolation(err error, c Constraint) bool {
	code, name, ok := pgErrorFields(err)
	return ok && code == "23502" && name == string(c)
}

// IsCheckViolation checks if err is a CHECK constraint violation for the given constraint.
func IsCheckViolation(err error, c Constraint) bool {
	code, name, ok := pgErrorFields(err)
	return ok && code == "23514" && name == string(c)
}

// IsConstraintViolation checks if err is any constraint violation for the given constraint,
// regardless of violation type.
func IsConstraintViolation(err error, c Constraint) bool {
	_, name, ok := pgErrorFields(err)
	return ok && name == string(c)
}

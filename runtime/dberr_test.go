package runtime

import (
	"fmt"
	"testing"
)

// fakePgxError mimics pgx's pgconn.PgError struct fields.
type fakePgxError struct {
	Code           string
	ConstraintName string
	Message        string
}

func (e *fakePgxError) Error() string { return e.Message }

// fakePqError mimics lib/pq's pq.Error struct fields.
type fakePqError struct {
	Code       string
	Constraint string
	Message    string
}

func (e *fakePqError) Error() string { return e.Message }

func TestIsUniqueViolation(t *testing.T) {
	err := &fakePgxError{Code: "23505", ConstraintName: "users_email_key", Message: "duplicate key"}

	if !IsUniqueViolation(err, "users_email_key") {
		t.Error("expected unique violation match")
	}
	if IsUniqueViolation(err, "other_constraint") {
		t.Error("should not match different constraint")
	}
	if IsForeignKeyViolation(err, "users_email_key") {
		t.Error("should not match as FK violation")
	}
}

func TestIsForeignKeyViolation(t *testing.T) {
	err := &fakePgxError{Code: "23503", ConstraintName: "posts_user_id_fkey", Message: "FK violation"}
	if !IsForeignKeyViolation(err, "posts_user_id_fkey") {
		t.Error("expected FK violation match")
	}
}

func TestIsNotNullViolation(t *testing.T) {
	err := &fakePgxError{Code: "23502", ConstraintName: "users_name_not_null", Message: "not null"}
	if !IsNotNullViolation(err, "users_name_not_null") {
		t.Error("expected not-null violation match")
	}
}

func TestIsCheckViolation(t *testing.T) {
	err := &fakePgxError{Code: "23514", ConstraintName: "users_age_check", Message: "check"}
	if !IsCheckViolation(err, "users_age_check") {
		t.Error("expected check violation match")
	}
}

func TestIsConstraintViolation(t *testing.T) {
	err := &fakePgxError{Code: "23505", ConstraintName: "users_email_key", Message: "dup"}
	if !IsConstraintViolation(err, "users_email_key") {
		t.Error("expected constraint violation match")
	}
}

func TestPqStyleError(t *testing.T) {
	err := &fakePqError{Code: "23505", Constraint: "users_email_key", Message: "duplicate"}
	if !IsUniqueViolation(err, "users_email_key") {
		t.Error("expected pq-style unique violation match")
	}
}

func TestWrappedError(t *testing.T) {
	inner := &fakePgxError{Code: "23505", ConstraintName: "users_email_key", Message: "dup"}
	wrapped := fmt.Errorf("insert failed: %w", inner)
	if !IsUniqueViolation(wrapped, "users_email_key") {
		t.Error("should match through wrapped error")
	}
}

func TestNonPgError(t *testing.T) {
	err := fmt.Errorf("some random error")
	if IsUniqueViolation(err, "anything") {
		t.Error("should not match non-PG error")
	}
}

func TestNilError(t *testing.T) {
	if IsUniqueViolation(nil, "anything") {
		t.Error("should not match nil")
	}
}

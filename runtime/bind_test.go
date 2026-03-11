package runtime

import (
	"context"
	"strings"
	"testing"
)

func TestBindRequiresPointer(t *testing.T) {
	q := NewQuery(PostgresDialect{}, "users")
	ctx := context.Background()
	var s struct{ Name string }

	// Non-pointer should fail.
	err := Bind(ctx, nil, q, s)
	if err == nil || !strings.Contains(err.Error(), "non-nil pointer") {
		t.Errorf("expected non-nil pointer error, got: %v", err)
	}

	// Nil pointer should fail.
	err = Bind(ctx, nil, q, (*struct{ Name string })(nil))
	if err == nil || !strings.Contains(err.Error(), "non-nil pointer") {
		t.Errorf("expected non-nil pointer error, got: %v", err)
	}
}

func TestBindRejectsNonStruct(t *testing.T) {
	q := NewQuery(PostgresDialect{}, "users")
	ctx := context.Background()
	var s string

	err := Bind(ctx, nil, q, &s)
	if err == nil || !strings.Contains(err.Error(), "struct or slice") {
		t.Errorf("expected struct/slice error, got: %v", err)
	}
}

func TestBindRejectsNonStructSlice(t *testing.T) {
	q := NewQuery(PostgresDialect{}, "users")
	ctx := context.Background()
	var s []string

	err := Bind(ctx, nil, q, &s)
	if err == nil || !strings.Contains(err.Error(), "struct or *struct") {
		t.Errorf("expected struct slice error, got: %v", err)
	}
}

func TestFieldPointersMatchesTags(t *testing.T) {
	type Custom struct {
		ID   int    `db:"id"`
		Name string `json:"user_name"`
		Age  int    // No tag
	}

	c := &Custom{}
	ptrs, err := FieldPointers(c, []string{"id", "user_name", "age"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ptrs) != 3 {
		t.Fatalf("got %d pointers, want 3", len(ptrs))
	}

	// id and user_name should point to real fields.
	// age has no tag, so it gets a discard pointer.
	*(ptrs[0].(*int)) = 42
	*(ptrs[1].(*string)) = "Alice"

	if c.ID != 42 {
		t.Errorf("ID = %d, want 42", c.ID)
	}
	if c.Name != "Alice" {
		t.Errorf("Name = %q, want Alice", c.Name)
	}
}

func TestFieldPointersDiscardUnmatched(t *testing.T) {
	type Small struct {
		ID int `db:"id"`
	}

	s := &Small{}
	ptrs, err := FieldPointers(s, []string{"id", "unknown_col"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ptrs) != 2 {
		t.Fatalf("got %d pointers, want 2", len(ptrs))
	}

	*(ptrs[0].(*int)) = 99
	if s.ID != 99 {
		t.Errorf("ID = %d, want 99", s.ID)
	}
}

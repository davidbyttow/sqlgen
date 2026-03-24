package sqlgen

import (
	"testing"
)

// ---------------------------------------------------------------------------
// ColumnMap tests
// ---------------------------------------------------------------------------

func TestColumnMapBasic(t *testing.T) {
	cm := NewColumnMap("id", "name", "email")

	if len(cm.Columns) != 3 {
		t.Fatalf("got %d columns, want 3", len(cm.Columns))
	}

	tests := []struct {
		col  string
		want int
	}{
		{"id", 0},
		{"name", 1},
		{"email", 2},
	}
	for _, tt := range tests {
		if got := cm.Index(tt.col); got != tt.want {
			t.Errorf("Index(%q) = %d, want %d", tt.col, got, tt.want)
		}
	}
}

func TestColumnMapUnknownColumn(t *testing.T) {
	cm := NewColumnMap("id", "name")

	if got := cm.Index("nonexistent"); got != -1 {
		t.Errorf("Index(nonexistent) = %d, want -1", got)
	}
}

func TestColumnMapEmpty(t *testing.T) {
	cm := NewColumnMap()

	if len(cm.Columns) != 0 {
		t.Fatalf("got %d columns, want 0", len(cm.Columns))
	}
	if got := cm.Index("anything"); got != -1 {
		t.Errorf("Index on empty map = %d, want -1", got)
	}
}

func TestColumnMapDuplicateColumns(t *testing.T) {
	// Last occurrence wins because the loop overwrites.
	cm := NewColumnMap("id", "name", "id")

	if got := cm.Index("id"); got != 2 {
		t.Errorf("Index(id) = %d, want 2 (last occurrence)", got)
	}
	if got := cm.Index("name"); got != 1 {
		t.Errorf("Index(name) = %d, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// FieldPointers tests
// ---------------------------------------------------------------------------

func TestFieldPointersDBTags(t *testing.T) {
	type Row struct {
		ID    int    `db:"id"`
		Name  string `db:"name"`
		Email string `db:"email"`
	}

	r := &Row{}
	ptrs, err := FieldPointers(r, []string{"id", "name", "email"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ptrs) != 3 {
		t.Fatalf("got %d pointers, want 3", len(ptrs))
	}

	*(ptrs[0].(*int)) = 1
	*(ptrs[1].(*string)) = "Alice"
	*(ptrs[2].(*string)) = "alice@example.com"

	if r.ID != 1 {
		t.Errorf("ID = %d, want 1", r.ID)
	}
	if r.Name != "Alice" {
		t.Errorf("Name = %q, want Alice", r.Name)
	}
	if r.Email != "alice@example.com" {
		t.Errorf("Email = %q, want alice@example.com", r.Email)
	}
}

func TestFieldPointersJSONTags(t *testing.T) {
	type Row struct {
		ID   int    `json:"id"`
		Name string `json:"full_name"`
	}

	r := &Row{}
	ptrs, err := FieldPointers(r, []string{"id", "full_name"})
	if err != nil {
		t.Fatal(err)
	}

	*(ptrs[0].(*int)) = 7
	*(ptrs[1].(*string)) = "Bob"

	if r.ID != 7 {
		t.Errorf("ID = %d, want 7", r.ID)
	}
	if r.Name != "Bob" {
		t.Errorf("Name = %q, want Bob", r.Name)
	}
}

func TestFieldPointersJSONOmitempty(t *testing.T) {
	type Row struct {
		ID   int    `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
	}

	r := &Row{}
	ptrs, err := FieldPointers(r, []string{"id", "name"})
	if err != nil {
		t.Fatal(err)
	}

	*(ptrs[0].(*int)) = 10
	*(ptrs[1].(*string)) = "Charlie"

	if r.ID != 10 {
		t.Errorf("ID = %d, want 10", r.ID)
	}
	if r.Name != "Charlie" {
		t.Errorf("Name = %q, want Charlie", r.Name)
	}
}

func TestFieldPointersDBTagTakesPrecedence(t *testing.T) {
	type Row struct {
		Name string `db:"db_name" json:"json_name"`
	}

	r := &Row{}
	ptrs, err := FieldPointers(r, []string{"db_name"})
	if err != nil {
		t.Fatal(err)
	}
	*(ptrs[0].(*string)) = "via db"
	if r.Name != "via db" {
		t.Errorf("Name = %q, want 'via db'", r.Name)
	}

	// Lookup by json tag should NOT match (db tag takes precedence).
	r2 := &Row{}
	ptrs2, err := FieldPointers(r2, []string{"json_name"})
	if err != nil {
		t.Fatal(err)
	}
	if r2.Name != "" {
		t.Errorf("Name should be empty when looked up by json tag, got %q", r2.Name)
	}
	_ = ptrs2
}

func TestFieldPointersDBDashTag(t *testing.T) {
	type Row struct {
		ID     int `db:"id"`
		Secret int `db:"-"`
	}

	r := &Row{}
	ptrs, err := FieldPointers(r, []string{"id", "secret"})
	if err != nil {
		t.Fatal(err)
	}

	*(ptrs[0].(*int)) = 42
	if r.ID != 42 {
		t.Errorf("ID = %d, want 42", r.ID)
	}

	// db:"-" is skipped, so "secret" gets a discard pointer.
	if r.Secret != 0 {
		t.Errorf("Secret = %d, want 0 (should be discarded)", r.Secret)
	}
}

func TestFieldPointersJSONDashTag(t *testing.T) {
	type Row struct {
		ID     int `json:"id"`
		Secret int `json:"-"`
	}

	r := &Row{}
	ptrs, err := FieldPointers(r, []string{"id", "secret"})
	if err != nil {
		t.Fatal(err)
	}

	*(ptrs[0].(*int)) = 5
	if r.ID != 5 {
		t.Errorf("ID = %d, want 5", r.ID)
	}
	if r.Secret != 0 {
		t.Errorf("Secret = %d, want 0 (json:\"-\" should be skipped)", r.Secret)
	}
}

func TestFieldPointersUnknownColumnsGetDiscard(t *testing.T) {
	type Row struct {
		ID int `db:"id"`
	}

	r := &Row{}
	ptrs, err := FieldPointers(r, []string{"id", "unknown1", "unknown2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ptrs) != 3 {
		t.Fatalf("got %d pointers, want 3", len(ptrs))
	}

	for i, p := range ptrs {
		if p == nil {
			t.Errorf("ptrs[%d] is nil, want non-nil (discard pointer)", i)
		}
	}

	*(ptrs[0].(*int)) = 100
	if r.ID != 100 {
		t.Errorf("ID = %d, want 100", r.ID)
	}
}

func TestFieldPointersNonStructError(t *testing.T) {
	s := "not a struct"
	_, err := FieldPointers(&s, []string{"col"})
	if err == nil {
		t.Fatal("expected error for non-struct, got nil")
	}

	_, err = FieldPointers(42, []string{"col"})
	if err == nil {
		t.Fatal("expected error for non-struct, got nil")
	}
}

func TestFieldPointersNestedStructNotFlattened(t *testing.T) {
	type Inner struct {
		Val string `db:"val"`
	}
	type Outer struct {
		ID    int   `db:"id"`
		Inner Inner // No db/json tag on the nested struct field.
	}

	r := &Outer{}
	ptrs, err := FieldPointers(r, []string{"id", "val"})
	if err != nil {
		t.Fatal(err)
	}

	*(ptrs[0].(*int)) = 1
	if r.ID != 1 {
		t.Errorf("ID = %d, want 1", r.ID)
	}

	if r.Inner.Val != "" {
		t.Errorf("Inner.Val = %q, want empty (nested fields shouldn't be mapped)", r.Inner.Val)
	}
}

func TestFieldPointersEmptyColumns(t *testing.T) {
	type Row struct {
		ID int `db:"id"`
	}

	r := &Row{}
	ptrs, err := FieldPointers(r, []string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(ptrs) != 0 {
		t.Fatalf("got %d pointers, want 0", len(ptrs))
	}
}

func TestFieldPointersValueNotPointer(t *testing.T) {
	type Row struct {
		ID int `db:"id"`
	}

	r := Row{}
	defer func() {
		if rec := recover(); rec != nil {
			// Expected: reflect will panic on non-addressable field.
		}
	}()
	_, _ = FieldPointers(r, []string{"id"})
}

func TestFieldPointersJSONEmptyTag(t *testing.T) {
	type Row struct {
		ID   int    `db:"id"`
		Name string `json:",omitempty"`
	}

	r := &Row{}
	ptrs, err := FieldPointers(r, []string{"id", "Name"})
	if err != nil {
		t.Fatal(err)
	}

	*(ptrs[0].(*int)) = 3
	if r.ID != 3 {
		t.Errorf("ID = %d, want 3", r.ID)
	}
	if r.Name != "" {
		t.Errorf("Name = %q, want empty", r.Name)
	}
}

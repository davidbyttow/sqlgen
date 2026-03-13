package sqlgen

import (
	"reflect"
	"testing"
)

func TestWhitelist(t *testing.T) {
	cols := []string{"id", "email", "name", "bio", "created_at"}
	vals := []any{1, "a@b.com", "Alice", nil, "2024-01-01"}

	gotCols, gotVals := FilterColumns(cols, vals, Whitelist("email", "name"))
	wantCols := []string{"email", "name"}
	wantVals := []any{"a@b.com", "Alice"}

	if !reflect.DeepEqual(gotCols, wantCols) {
		t.Errorf("cols = %v, want %v", gotCols, wantCols)
	}
	if !reflect.DeepEqual(gotVals, wantVals) {
		t.Errorf("vals = %v, want %v", gotVals, wantVals)
	}
}

func TestBlacklist(t *testing.T) {
	cols := []string{"id", "email", "name", "bio", "created_at"}
	vals := []any{1, "a@b.com", "Alice", nil, "2024-01-01"}

	gotCols, gotVals := FilterColumns(cols, vals, Blacklist("id", "created_at"))
	wantCols := []string{"email", "name", "bio"}
	wantVals := []any{"a@b.com", "Alice", nil}

	if !reflect.DeepEqual(gotCols, wantCols) {
		t.Errorf("cols = %v, want %v", gotCols, wantCols)
	}
	if !reflect.DeepEqual(gotVals, wantVals) {
		t.Errorf("vals = %v, want %v", gotVals, wantVals)
	}
}

func TestFilterColumnsNoFilter(t *testing.T) {
	cols := []string{"id", "email"}
	vals := []any{1, "a@b.com"}

	gotCols, gotVals := FilterColumns(cols, vals)
	if !reflect.DeepEqual(gotCols, cols) {
		t.Errorf("cols = %v, want %v", gotCols, cols)
	}
	if !reflect.DeepEqual(gotVals, vals) {
		t.Errorf("vals = %v, want %v", gotVals, vals)
	}
}

func TestFilterColumnsZeroValue(t *testing.T) {
	cols := []string{"id", "email"}
	vals := []any{1, "a@b.com"}

	gotCols, gotVals := FilterColumns(cols, vals, Columns{})
	if !reflect.DeepEqual(gotCols, cols) {
		t.Errorf("cols = %v, want %v", gotCols, cols)
	}
	if !reflect.DeepEqual(gotVals, vals) {
		t.Errorf("vals = %v, want %v", gotVals, vals)
	}
}

func TestWhitelistEmpty(t *testing.T) {
	cols := []string{"id", "email"}
	vals := []any{1, "a@b.com"}

	gotCols, gotVals := FilterColumns(cols, vals, Whitelist())
	if len(gotCols) != 0 {
		t.Errorf("expected empty cols, got %v", gotCols)
	}
	if len(gotVals) != 0 {
		t.Errorf("expected empty vals, got %v", gotVals)
	}
}

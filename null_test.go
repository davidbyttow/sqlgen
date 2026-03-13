package sqlgen

import (
	"encoding/json"
	"testing"
)

func TestNullZeroValue(t *testing.T) {
	var n Null[string]
	if n.Valid {
		t.Error("zero value should be invalid (null)")
	}
	if n.Val != "" {
		t.Error("zero value Val should be empty string")
	}
}

func TestNewNull(t *testing.T) {
	n := NewNull("hello")
	if !n.Valid {
		t.Error("NewNull should be valid")
	}
	if n.Val != "hello" {
		t.Errorf("Val = %q, want hello", n.Val)
	}
}

func TestNullSetClear(t *testing.T) {
	var n Null[int]
	n.Set(42)
	if !n.Valid || n.Val != 42 {
		t.Errorf("after Set: Valid=%v Val=%v", n.Valid, n.Val)
	}
	n.Clear()
	if n.Valid || n.Val != 0 {
		t.Errorf("after Clear: Valid=%v Val=%v", n.Valid, n.Val)
	}
}

func TestNullPtr(t *testing.T) {
	var null Null[string]
	if null.Ptr() != nil {
		t.Error("null Ptr() should be nil")
	}

	set := NewNull("test")
	p := set.Ptr()
	if p == nil || *p != "test" {
		t.Errorf("Ptr() = %v, want *test", p)
	}
}

func TestFromPtr(t *testing.T) {
	n := FromPtr[string](nil)
	if n.Valid {
		t.Error("FromPtr(nil) should be invalid")
	}

	s := "hello"
	n = FromPtr(&s)
	if !n.Valid || n.Val != "hello" {
		t.Errorf("FromPtr(&hello) = %+v", n)
	}
}

func TestNullJSONRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		val  Null[string]
		json string
	}{
		{"null", Null[string]{}, "null"},
		{"value", NewNull("hello"), `"hello"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.val)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}
			if string(data) != tt.json {
				t.Errorf("Marshal = %s, want %s", data, tt.json)
			}

			var got Null[string]
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal error: %v", err)
			}
			if got.Valid != tt.val.Valid || got.Val != tt.val.Val {
				t.Errorf("Unmarshal = %+v, want %+v", got, tt.val)
			}
		})
	}
}

func TestNullJSONInt(t *testing.T) {
	n := NewNull(42)
	data, err := json.Marshal(n)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "42" {
		t.Errorf("Marshal int = %s, want 42", data)
	}

	var got Null[int]
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Valid || got.Val != 42 {
		t.Errorf("Unmarshal int = %+v", got)
	}
}

func TestNullJSONBool(t *testing.T) {
	n := NewNull(true)
	data, _ := json.Marshal(n)
	if string(data) != "true" {
		t.Errorf("Marshal bool = %s", data)
	}
}

func TestNullJSONInStruct(t *testing.T) {
	type User struct {
		Name  string      `json:"name"`
		Email Null[string] `json:"email"`
	}

	u := User{Name: "Alice", Email: NewNull("alice@example.com")}
	data, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}

	var got User
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Email.Val != "alice@example.com" || !got.Email.Valid {
		t.Errorf("round-trip email = %+v", got.Email)
	}

	// Test with null email.
	u2 := User{Name: "Bob"}
	data2, _ := json.Marshal(u2)
	var got2 User
	json.Unmarshal(data2, &got2)
	if got2.Email.Valid {
		t.Error("null email should be invalid after round-trip")
	}
}

func TestNullString(t *testing.T) {
	if NewNull("hi").String() != "hi" {
		t.Error("String() for valid")
	}
	var n Null[string]
	if n.String() != "NULL" {
		t.Error("String() for null")
	}
}

package sqlgen

import (
	"database/sql/driver"
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

func TestNullInt32Value(t *testing.T) {
	n := NewNull(int32(42))
	v, err := n.Value()
	if err != nil {
		t.Fatal(err)
	}
	// driver.Value must be int64, not int32.
	i64, ok := v.(int64)
	if !ok {
		t.Fatalf("Value() returned %T, want int64", v)
	}
	if i64 != 42 {
		t.Errorf("Value() = %d, want 42", i64)
	}
}

func TestNullFloat32Value(t *testing.T) {
	n := NewNull(float32(3.14))
	v, err := n.Value()
	if err != nil {
		t.Fatal(err)
	}
	f64, ok := v.(float64)
	if !ok {
		t.Fatalf("Value() returned %T, want float64", v)
	}
	if f64 < 3.13 || f64 > 3.15 {
		t.Errorf("Value() = %f, want ~3.14", f64)
	}
}

func TestNullNilValue(t *testing.T) {
	var n Null[int32]
	v, err := n.Value()
	if err != nil {
		t.Fatal(err)
	}
	if v != nil {
		t.Errorf("null Value() = %v, want nil", v)
	}
}

func TestNullValueTypes(t *testing.T) {
	tests := []struct {
		name string
		val  driver.Valuer
		want any // expected type
	}{
		{"int8", NewNull(int8(1)), int64(1)},
		{"int16", NewNull(int16(2)), int64(2)},
		{"int32", NewNull(int32(3)), int64(3)},
		{"int", NewNull(int(4)), int64(4)},
		{"uint8", NewNull(uint8(5)), int64(5)},
		{"uint16", NewNull(uint16(6)), int64(6)},
		{"uint32", NewNull(uint32(7)), int64(7)},
		{"float32", NewNull(float32(1.5)), float64(1.5)},
		{"string", NewNull("hello"), "hello"},
		{"bool", NewNull(true), true},
		{"int64", NewNull(int64(99)), int64(99)},
		{"float64", NewNull(float64(2.5)), float64(2.5)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := tt.val.Value()
			if err != nil {
				t.Fatal(err)
			}
			if v != tt.want {
				t.Errorf("Value() = %v (%T), want %v (%T)", v, v, tt.want, tt.want)
			}
		})
	}
}

func TestNullScanBytesIntoRawMessage(t *testing.T) {
	// json.RawMessage is defined as []byte but is a distinct type.
	// Drivers return []byte for JSONB, so Scan must handle the conversion.
	type RawMessage = json.RawMessage
	var n Null[RawMessage]
	src := []byte(`{"key":"value"}`)
	if err := n.Scan(src); err != nil {
		t.Fatalf("Scan([]byte) into Null[RawMessage]: %v", err)
	}
	if !n.Valid {
		t.Fatal("expected Valid after Scan")
	}
	if string(n.Val) != `{"key":"value"}` {
		t.Errorf("Val = %s, want {\"key\":\"value\"}", n.Val)
	}
}

func TestNullScanInt64IntoInt32(t *testing.T) {
	var n Null[int32]
	// Drivers return int64, Scan must convert to int32.
	if err := n.Scan(int64(42)); err != nil {
		t.Fatalf("Scan(int64) into Null[int32]: %v", err)
	}
	if !n.Valid || n.Val != 42 {
		t.Errorf("got Valid=%v Val=%v, want Valid=true Val=42", n.Valid, n.Val)
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

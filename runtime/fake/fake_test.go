package fake

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestString(t *testing.T) {
	s := String("name")
	if !strings.HasPrefix(s, "name_") {
		t.Errorf("String = %q, want prefix name_", s)
	}
	if len(s) < 6 {
		t.Errorf("String too short: %q", s)
	}
	// Two calls should produce different values.
	s2 := String("name")
	if s == s2 {
		t.Errorf("two calls produced same value: %q", s)
	}
}

func TestInt16(t *testing.T) {
	v := Int16()
	if v < 1 || v > 32000 {
		t.Errorf("Int16 = %d, out of range", v)
	}
}

func TestInt32(t *testing.T) {
	v := Int32()
	if v < 1 || v > 2_000_000 {
		t.Errorf("Int32 = %d, out of range", v)
	}
}

func TestInt64(t *testing.T) {
	v := Int64()
	if v < 1 || v > 2_000_000_000 {
		t.Errorf("Int64 = %d, out of range", v)
	}
}

func TestFloat32(t *testing.T) {
	v := Float32()
	if v < 0.01 || v >= 1000.0 {
		t.Errorf("Float32 = %f, out of range", v)
	}
}

func TestFloat64(t *testing.T) {
	v := Float64()
	if v < 0.01 || v >= 1000.0 {
		t.Errorf("Float64 = %f, out of range", v)
	}
}

func TestBool(t *testing.T) {
	// Run enough times to see both values.
	seen := map[bool]bool{}
	for range 100 {
		seen[Bool()] = true
	}
	if len(seen) != 2 {
		t.Errorf("Bool only produced %d distinct values in 100 calls", len(seen))
	}
}

func TestTime(t *testing.T) {
	v := Time()
	if v.After(time.Now()) {
		t.Errorf("Time = %v, should be in the past", v)
	}
	if v.Before(time.Now().Add(-366 * 24 * time.Hour)) {
		t.Errorf("Time = %v, too far in the past", v)
	}
}

func TestUUID(t *testing.T) {
	u := UUID()
	if len(u) != 36 {
		t.Errorf("UUID length = %d, want 36", len(u))
	}
	// Check format: 8-4-4-4-12
	parts := strings.Split(u, "-")
	if len(parts) != 5 {
		t.Fatalf("UUID parts = %d, want 5", len(parts))
	}
	// Version nibble should be 4.
	if parts[2][0] != '4' {
		t.Errorf("UUID version = %c, want 4", parts[2][0])
	}
}

func TestBytes(t *testing.T) {
	b := Bytes(16)
	if len(b) != 16 {
		t.Errorf("Bytes length = %d, want 16", len(b))
	}
}

func TestJSON(t *testing.T) {
	j := JSON()
	if !json.Valid(j) {
		t.Errorf("JSON invalid: %s", j)
	}
}

func TestNumeric(t *testing.T) {
	n := Numeric()
	if !strings.Contains(n, ".") {
		t.Errorf("Numeric = %q, expected decimal", n)
	}
}

func TestPtr(t *testing.T) {
	s := "hello"
	p := Ptr(s)
	if *p != "hello" {
		t.Errorf("Ptr = %q, want hello", *p)
	}
}

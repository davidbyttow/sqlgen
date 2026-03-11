// Package fake provides random value generators for use in generated factory functions.
package fake

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"time"
)

// String returns a random string with the given prefix.
func String(prefix string) string {
	return fmt.Sprintf("%s_%s", prefix, hex8())
}

// Int16 returns a random int16 in [1, 32000].
func Int16() int16 {
	return int16(rand.IntN(31999)) + 1
}

// Int32 returns a random int32 in [1, 2_000_000].
func Int32() int32 {
	return int32(rand.IntN(1_999_999)) + 1
}

// Int64 returns a random int64 in [1, 2_000_000_000].
func Int64() int64 {
	return int64(rand.IntN(1_999_999_999)) + 1
}

// Float32 returns a random float32 in [0.01, 1000.0).
func Float32() float32 {
	return float32(rand.Float64()*999.99) + 0.01
}

// Float64 returns a random float64 in [0.01, 1000.0).
func Float64() float64 {
	return rand.Float64()*999.99 + 0.01
}

// Bool returns a random bool.
func Bool() bool {
	return rand.IntN(2) == 1
}

// Time returns a random time in the last 365 days, truncated to microseconds
// (Postgres precision).
func Time() time.Time {
	offset := time.Duration(rand.IntN(365*24)) * time.Hour
	return time.Now().Add(-offset).Truncate(time.Microsecond)
}

// UUID returns a random UUID v4 string.
func UUID() string {
	var b [16]byte
	for i := range b {
		b[i] = byte(rand.IntN(256))
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 1
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Bytes returns n random bytes.
func Bytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(rand.IntN(256))
	}
	return b
}

// JSON returns a minimal valid JSON object.
func JSON() json.RawMessage {
	return json.RawMessage(`{}`)
}

// Numeric returns a random decimal string (for numeric/money columns).
func Numeric() string {
	return fmt.Sprintf("%d.%02d", rand.IntN(10000), rand.IntN(100))
}

// Ptr returns a pointer to the given value.
func Ptr[T any](v T) *T {
	return &v
}

func hex8() string {
	var b [4]byte
	for i := range b {
		b[i] = byte(rand.IntN(256))
	}
	return hex.EncodeToString(b[:])
}

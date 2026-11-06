package ids

import (
	"strings"
	"testing"
	"time"
)

func TestShapeAndAlphabet(t *testing.T) {
	id := New()
	if len(id) != 26 {
		t.Fatalf("len(%q) = %d, want 26", id, len(id))
	}
	for _, r := range id {
		if !strings.ContainsRune(alphabet, r) {
			t.Fatalf("%q has %q, outside Crockford base32", id, r)
		}
	}
	if id[0] > '7' {
		t.Fatalf("%q: the first digit only carries 3 bits", id)
	}
}

func TestSortsByTime(t *testing.T) {
	t0 := time.Date(2026, 11, 6, 8, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		before time.Time
		after  time.Time
	}{
		{"one millisecond", t0, t0.Add(time.Millisecond)},
		{"one day", t0, t0.Add(24 * time.Hour)},
		{"years", time.UnixMilli(0), t0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, b := At(c.before), At(c.after)
			if a >= b {
				t.Fatalf("%s should sort before %s", a, b)
			}
		})
	}
}

func TestKnownTimestamp(t *testing.T) {
	// the first 10 digits are the millisecond timestamp; 0 ms encodes as ten zeros
	if got := At(time.UnixMilli(0))[:10]; got != "0000000000" {
		t.Fatalf("timestamp digits = %q", got)
	}
	// 2^48-1 ms (the largest ULID time) is 7ZZZZZZZZZ
	if got := At(time.UnixMilli(1<<48 - 1))[:10]; got != "7ZZZZZZZZZ" {
		t.Fatalf("timestamp digits = %q", got)
	}
}

func TestUnique(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for range 1000 {
		id := New()
		if seen[id] {
			t.Fatalf("duplicate %s", id)
		}
		seen[id] = true
	}
}

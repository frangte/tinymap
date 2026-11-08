// Package ids makes the persistent ids (accounts, characters): ULIDs, 26 characters of Crockford base32
// that sort by creation time. The wire keeps using small per-connection numbers; these are for storage.
package ids

import (
	"crypto/rand"
	"encoding/binary"
	"time"
)

const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// New returns a ULID for the current time.
func New() string { return At(time.Now()) }

// At returns a ULID with the timestamp t (millisecond precision) followed by 80 random bits.
func At(t time.Time) string {
	var b [16]byte
	ms := uint64(t.UnixMilli())
	for i := 5; i >= 0; i-- {
		b[i] = byte(ms)
		ms >>= 8
	}
	_, _ = rand.Read(b[6:]) // crypto/rand.Read never fails: it crashes the program instead
	return encode(b)
}

// encode writes the 128 bits as 26 base32 digits, most significant first (the first digit carries 3 bits).
func encode(b [16]byte) string {
	hi := binary.BigEndian.Uint64(b[:8])
	lo := binary.BigEndian.Uint64(b[8:])
	var out [26]byte
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = alphabet[lo&31]
		lo = lo>>5 | hi<<59
		hi >>= 5
	}
	return string(out[:])
}

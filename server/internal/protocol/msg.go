// Package protocol defines the JSON messages between the game client and the server, and validates
// everything a client sends. server/README.md is the spec; src/net/protocol.ts mirrors it.
package protocol

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"unicode"
)

// Limits shared with the client (keep in sync with src/game/profile.ts and src/net/protocol.ts).
const (
	NameMax = 16
	ChatMax = 140
	// MaxMessageBytes caps one client frame; the largest legal one (a 140-character chat) is far below it.
	MaxMessageBytes = 1024
	// WorldHalf bounds |x| and |z| in three.js metres; the island spans roughly -120…130.
	WorldHalf = 400
	MinY      = -20
	MaxY      = 120
)

// Message types (the "t" field).
const (
	TypeHello   = "hello"
	TypeState   = "state"
	TypeProfile = "profile"
	TypeChat    = "chat"
	TypeWelcome = "welcome"
	TypeJoin    = "join"
	TypeLeave   = "leave"
	TypeSnap    = "snap"
	TypeError   = "error"
)

// Error codes sent in {"t":"error"}.
const (
	ErrFull     = "full"
	ErrBadHello = "bad_hello"
	ErrRate     = "rate"
	ErrSlow     = "slow"
)

var (
	bodies  = map[string]bool{"man": true, "woman": true}
	outfits = map[string]bool{"thun": true, "aodai": true, "baba": true, "somi": true, "xeom": true, "bikini": true}
	hats    = map[string]bool{"nonla": true, "cap": true, "none": true}
)

// Look is what a pedestrian wears (src/game/outfits.ts).
type Look struct {
	Body   string `json:"body"`
	Outfit string `json:"outfit"`
	Hat    string `json:"hat"`
}

// Valid reports whether every field is a known id.
func (l Look) Valid() bool {
	return bodies[l.Body] && outfits[l.Outfit] && hats[l.Hat]
}

// Pose is a player's position (three.js metres, Y up) and heading (radians).
type Pose struct {
	X, Y, Z, H float64
}

// ClientMsg is any message a client sends; which fields matter depends on T.
type ClientMsg struct {
	T    string  `json:"t"`
	Name string  `json:"name"`
	Look *Look   `json:"look"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Z    float64 `json:"z"`
	H    float64 `json:"h"`
	Text string  `json:"text"`
}

// Validation errors; the hub drops the message (or the connection, for a bad hello).
var (
	ErrUnknownType = errors.New("unknown message type")
	ErrBadLook     = errors.New("invalid look")
	ErrBadPose     = errors.New("pose out of bounds")
	ErrEmptyChat   = errors.New("empty chat")
)

// Decode parses one client frame and normalises it: names and chat are cleaned, the heading is wrapped.
func Decode(data []byte) (ClientMsg, error) {
	var m ClientMsg
	if err := json.Unmarshal(data, &m); err != nil {
		return m, err
	}
	switch m.T {
	case TypeHello, TypeProfile:
		if m.Look == nil || !m.Look.Valid() {
			return m, ErrBadLook
		}
		m.Name = CleanName(m.Name)
	case TypeState:
		if !finite(m.X, m.Y, m.Z, m.H) || math.Abs(m.X) > WorldHalf || math.Abs(m.Z) > WorldHalf || m.Y < MinY || m.Y > MaxY {
			return m, ErrBadPose
		}
		m.H = math.Atan2(math.Sin(m.H), math.Cos(m.H))
	case TypeChat:
		m.Text = CleanChat(m.Text)
		if m.Text == "" {
			return m, ErrEmptyChat
		}
	default:
		return m, ErrUnknownType
	}
	return m, nil
}

func finite(vs ...float64) bool {
	for _, v := range vs {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

// clean drops control and invisible format characters (zero-width, bidi overrides), collapses whitespace to
// single spaces, trims, and caps the length in characters.
func clean(s string, max int) string {
	var b strings.Builder
	pendingSpace := false
	n := 0
	for _, r := range s {
		if unicode.IsSpace(r) {
			pendingSpace = n > 0
			continue
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == unicode.ReplacementChar {
			continue
		}
		need := 1
		if pendingSpace {
			need = 2
		}
		if n+need > max {
			break
		}
		if pendingSpace {
			b.WriteByte(' ')
			n++
			pendingSpace = false
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// CleanName normalises a display name (same rules as cleanName in src/game/profile.ts).
func CleanName(s string) string { return clean(s, NameMax) }

// CleanChat normalises a chat line to one line of at most ChatMax characters.
func CleanChat(s string) string { return clean(s, ChatMax) }

// ---- server -> client

// Player is a full description of someone in the world, sent on welcome and join.
// Pose is [x, y, z, h], absent until the player has sent its first state.
type Player struct {
	ID   uint32      `json:"id"`
	Name string      `json:"name"`
	Look Look        `json:"look"`
	Pose *[4]float64 `json:"pose,omitempty"`
}

// ChatLine is one chat message, live (with T set) or inside the welcome history.
type ChatLine struct {
	T    string `json:"t,omitempty"`
	ID   uint32 `json:"id"`
	Name string `json:"name"`
	Text string `json:"text"`
	At   int64  `json:"at"`
}

type Welcome struct {
	T          string     `json:"t"`
	ID         uint32     `json:"id"`
	Name       string     `json:"name"`
	MaxPlayers int        `json:"max"`
	Players    []Player   `json:"players"`
	Chat       []ChatLine `json:"chat"`
}

type Join struct {
	T string `json:"t"`
	P Player `json:"p"`
}

type Leave struct {
	T  string `json:"t"`
	ID uint32 `json:"id"`
}

type Profile struct {
	T    string `json:"t"`
	ID   uint32 `json:"id"`
	Name string `json:"name"`
	Look Look   `json:"look"`
}

// Snap carries every placed player as [id, x, y, z, h, age]: age is how many ms before this snapshot the
// server received that pose, so a client can place it on the sender's timeline instead of the snapshot tick's.
type Snap struct {
	T  string       `json:"t"`
	At int64        `json:"at"`
	P  [][6]float64 `json:"p"`
}

type Error struct {
	T    string `json:"t"`
	Code string `json:"code"`
}

// PoseArray rounds a pose for the wire: centimetres and milliradians are plenty.
func PoseArray(p Pose) [4]float64 {
	return [4]float64{round(p.X, 100), round(p.Y, 100), round(p.Z, 100), round(p.H, 1000)}
}

func round(v, k float64) float64 { return math.Round(v*k) / k }

// Encode marshals a server message. The message types above contain only strings, numbers and slices,
// so a failure here is a programming error.
func Encode(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic("protocol: unencodable message: " + err.Error())
	}
	return b
}

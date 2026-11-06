// Package protocol defines the JSON messages between the game client and the server, and validates
// everything a client sends. server/README.md is the spec; src/net/protocol.ts mirrors it.
package protocol

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// Limits shared with the client (keep in sync with src/game/profile.ts and src/net/protocol.ts).
const (
	NameMax = 16
	ChatMax = 140
	// TokenMax caps the account token in a hello; the server issues 43-character tokens (POST /api/auth/guest).
	TokenMax = 64
	// ArchMax caps the archetype id in a hello (ids are in server/content/archetypes.json)
	ArchMax = 32
	// MaxMessageBytes caps one client frame; the largest legal one (a 140-character chat) is far below it.
	MaxMessageBytes = 1024
	// WorldHalf bounds |x| and |z| in three.js metres; the island spans roughly -120…130.
	WorldHalf = 400
	MinY      = -20
	MaxY      = 120
)

// Message types (the "t" field).
const (
	TypeHello    = "hello"
	TypeState    = "state"
	TypeProfile  = "profile"
	TypeChat     = "chat"
	TypeMount    = "mount"
	TypeDismount = "dismount"
	TypeRespawn  = "respawn"
	TypeWelcome  = "welcome"
	TypeJoin     = "join"
	TypeLeave    = "leave"
	TypeSnap     = "snap"
	TypeCorrect  = "correct"
	TypeVehicle  = "vehicle"
	// TypeVehicleGone: a vehicle left the world (its owner did)
	TypeVehicleGone = "vehicle_gone"
	TypeError       = "error"
)

// State flags (the "s" field of a state, and of every snapshot row).
const (
	// FlagAir: in the air (jumping)
	FlagAir = 1 << 0
	// FlagRide: on a vehicle. The server sets it from its own record; what a client says is ignored.
	FlagRide = 1 << 1
	// EmoteShift and EmoteMask place the emote code (server/content/emotes.json, 0 for none) in bits 4–7.
	EmoteShift = 4
	EmoteMask  = 0xF << EmoteShift
	flagsKnown = FlagAir | FlagRide | EmoteMask
)

// EmoteOf is the emote code in state flags (0 for none).
func EmoteOf(s int) int { return (s & EmoteMask) >> EmoteShift }

// Error codes sent in {"t":"error"}.
const (
	ErrFull     = "full"
	ErrBadHello = "bad_hello"
	ErrRate     = "rate"
	ErrSlow     = "slow"
	// ErrAuth: the hello's token belongs to no account (the database was reset, or it was made up). The client
	// should forget the token, get a new one and connect again; the socket is closed after this.
	ErrAuth = "auth"
	// ErrMount: a mount was refused (not your vehicle, too far away, already riding)
	ErrMount = "mount"
	// ErrReplaced: the same character joined again from another connection (a reconnect after the network
	// dropped, or a second tab); this one is closed. The client should not reconnect on its own.
	ErrReplaced = "replaced"
)

// Chat channels (the "ch" field): global reaches everyone, nearby only the players within NearbyRadius.
const (
	ChannelGlobal = "global"
	ChannelNearby = "nearby"
)

// Who may wear crab and helmet is decided by capabilities (server/content/archetypes.json), not here.
var (
	bodies  = map[string]bool{"man": true, "woman": true}
	outfits = map[string]bool{"thun": true, "aodai": true, "baba": true, "somi": true, "xeom": true, "bikini": true, "crab": true}
	hats    = map[string]bool{"nonla": true, "cap": true, "none": true, "helmet": true}
)

// OutfitKnown and HatKnown say whether an id exists at all (for checking content files).
func OutfitKnown(id string) bool { return outfits[id] }
func HatKnown(id string) bool    { return hats[id] }

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
	T string `json:"t"`
	// Token is the account token, in hello only; empty for a guest without an account.
	Token string `json:"token"`
	Name  string `json:"name"`
	// Arch is the archetype the player picked, in hello only; the server keeps the one a character already has.
	Arch string `json:"arch"`
	// Ch is the chat channel, in chat only: "global" (also when empty, as older clients send) or "nearby".
	Ch   string  `json:"ch"`
	Look *Look   `json:"look"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Z    float64 `json:"z"`
	H    float64 `json:"h"`
	Text string  `json:"text"`
	// S is the state flags, in state only: FlagAir and the emote code
	S int `json:"s"`
	// K is how many corrections the client has applied, in state and respawn: one sent before the client applied
	// the server's latest correction is stale and ignored
	K uint32 `json:"k"`
	// V is the vehicle to get on, in mount only
	V uint32 `json:"v"`
}

// Validation errors; the world drops the message (or the connection, for a bad hello).
var (
	ErrUnknownType = errors.New("unknown message type")
	ErrBadLook     = errors.New("invalid look")
	ErrBadToken    = errors.New("token too long")
	ErrBadArch     = errors.New("archetype id too long")
	ErrBadChannel  = errors.New("unknown chat channel")
	ErrBadPose     = errors.New("pose out of bounds")
	ErrBadFlags    = errors.New("unknown state flags")
	ErrBadVehicle  = errors.New("no vehicle id")
	ErrEmptyChat   = errors.New("empty chat")
)

// Decode parses one client frame and normalises it: names and chat are cleaned, the heading is wrapped.
func Decode(data []byte) (ClientMsg, error) {
	var m ClientMsg
	if err := json.Unmarshal(data, &m); err != nil {
		return m, err
	}
	if m.T != TypeHello {
		m.Token, m.Arch = "", "" // only a hello carries the account and the archetype
	}
	if m.T != TypeChat {
		m.Ch = ""
	}
	if m.T != TypeState {
		m.S = 0
	}
	if m.T != TypeState && m.T != TypeRespawn {
		m.K = 0
	}
	if m.T != TypeMount {
		m.V = 0
	}
	switch m.T {
	case TypeHello, TypeProfile:
		if m.Look == nil || !m.Look.Valid() {
			return m, ErrBadLook
		}
		if len(m.Token) > TokenMax {
			return m, ErrBadToken
		}
		if len(m.Arch) > ArchMax {
			return m, ErrBadArch
		}
		m.Name = CleanName(m.Name)
	case TypeState, TypeRespawn:
		if !finite(m.X, m.Y, m.Z, m.H) || math.Abs(m.X) > WorldHalf || math.Abs(m.Z) > WorldHalf || m.Y < MinY || m.Y > MaxY {
			return m, ErrBadPose
		}
		if m.S < 0 || m.S&^flagsKnown != 0 {
			return m, ErrBadFlags
		}
		m.H = math.Atan2(math.Sin(m.H), math.Cos(m.H))
	case TypeMount:
		if m.V == 0 {
			return m, ErrBadVehicle
		}
	case TypeDismount:
	case TypeChat:
		switch m.Ch {
		case "":
			m.Ch = ChannelGlobal
		case ChannelGlobal, ChannelNearby:
		default:
			return m, ErrBadChannel
		}
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

// Player is a full description of someone in the world, sent on welcome and join to everyone. Where they are
// comes in snapshots, and only to the players near them.
type Player struct {
	ID   uint32 `json:"id"`
	Name string `json:"name"`
	// Arch is the archetype id, absent for a player who has none (an older client)
	Arch string `json:"arch,omitempty"`
	Look Look   `json:"look"`
}

// ChatLine is one chat message, live (with T set) or inside the welcome history (global lines only).
type ChatLine struct {
	T    string `json:"t,omitempty"`
	ID   uint32 `json:"id"`
	Name string `json:"name"`
	// Arch is the sender's archetype, so the line keeps its badge after they leave
	Arch string `json:"arch,omitempty"`
	Ch   string `json:"ch"`
	Text string `json:"text"`
	At   int64  `json:"at"`
}

type Welcome struct {
	T    string `json:"t"`
	ID   uint32 `json:"id"`
	Name string `json:"name"`
	// Arch is the player's archetype as the server keeps it (an archetype is chosen once); Apps the phone apps
	// its capabilities give, in phone order
	Arch       string   `json:"arch,omitempty"`
	Apps       []string `json:"apps"`
	MaxPlayers int      `json:"max"`
	// Pose is where the server placed the player: the last visit's position, or the archetype's start
	Pose     [4]float64 `json:"pose"`
	Players  []Player   `json:"players"`
	Vehicles []Vehicle  `json:"vehicles"`
	Chat     []ChatLine `json:"chat"`
}

// Vehicle is a vehicle in the world, sent to everyone on welcome and whenever it changes. While Rider is set it
// moves with the rider (whose snapshot rows carry FlagRide); Pose is where it was last parked.
type Vehicle struct {
	T     string     `json:"t,omitempty"`
	ID    uint32     `json:"id"`
	Kind  string     `json:"kind"`
	Owner uint32     `json:"owner"`
	Rider uint32     `json:"rider"`
	Pose  [4]float64 `json:"pose"`
}

type VehicleGone struct {
	T  string `json:"t"`
	ID uint32 `json:"id"`
}

// Correct tells a client its last move was refused: it goes back to Pose and counts the correction in K, which
// its states echo from then on.
type Correct struct {
	T    string     `json:"t"`
	K    uint32     `json:"k"`
	Pose [4]float64 `json:"pose"`
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

// A snapshot is {"t":"snap","at":ms,"p":[row…],"out":[id…]}, built by hand because it is the one message sent
// to every player twelve times a second. p has a row [id, x, y, z, h, s, age] for each player near the receiver
// that moved since the last snapshot or just came into view; age is how many ms before this snapshot the server
// received that pose, so a client can place it on the sender's timeline instead of the snapshot tick's. out lists
// the players that left the receiver's view (omitted when empty).

// AppendSnapRow appends one snapshot row.
func AppendSnapRow(b []byte, id uint32, p Pose, s int, ageMs int64) []byte {
	a := PoseArray(p)
	b = append(b, '[')
	b = strconv.AppendUint(b, uint64(id), 10)
	for _, v := range a {
		b = append(b, ',')
		b = strconv.AppendFloat(b, v, 'f', -1, 64)
	}
	b = append(b, ',')
	b = strconv.AppendInt(b, int64(s), 10)
	b = append(b, ',')
	b = strconv.AppendInt(b, max(0, ageMs), 10)
	return append(b, ']')
}

// EncodeSnap builds a snapshot from rows made by AppendSnapRow.
func EncodeSnap(at int64, rows [][]byte, out []uint32) []byte {
	n := 40 + 8*len(out)
	for _, r := range rows {
		n += len(r) + 1
	}
	b := make([]byte, 0, n)
	b = append(b, `{"t":"snap","at":`...)
	b = strconv.AppendInt(b, at, 10)
	b = append(b, `,"p":[`...)
	for i, r := range rows {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, r...)
	}
	b = append(b, ']')
	if len(out) > 0 {
		b = append(b, `,"out":[`...)
		for i, id := range out {
			if i > 0 {
				b = append(b, ',')
			}
			b = strconv.AppendUint(b, uint64(id), 10)
		}
		b = append(b, ']')
	}
	return append(b, '}')
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

package protocol

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestCleanName(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"keeps vietnamese", "Lan Anh", "Lan Anh"},
		{"trims and collapses spaces", "  Lan \t\n  Anh  ", "Lan Anh"},
		{"drops zero-width and bidi overrides", "Lan​‮Anh", "LanAnh"},
		{"drops control characters", "a\x00b\x1bc", "abc"},
		{"caps at 16 characters, not bytes", strings.Repeat("ữ", 20), strings.Repeat("ữ", 16)},
		{"does not end on a space when capping", "abcdefghijklmno pq", "abcdefghijklmno"},
		{"empty stays empty", " ​ ", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CleanName(c.in); got != c.want {
				t.Fatalf("CleanName(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestCleanChatCapsLength(t *testing.T) {
	got := CleanChat(strings.Repeat("a", 500))
	if n := len([]rune(got)); n != ChatMax {
		t.Fatalf("len = %d, want %d", n, ChatMax)
	}
}

func TestDecode(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr error
		check   func(t *testing.T, m ClientMsg)
	}{
		{name: "hello", in: `{"t":"hello","name":" Lan ","look":{"body":"woman","outfit":"aodai","hat":"nonla"}}`,
			check: func(t *testing.T, m ClientMsg) {
				if m.Name != "Lan" || m.Look.Outfit != "aodai" {
					t.Fatalf("got %+v", m)
				}
			}},
		{name: "swimwear", in: `{"t":"profile","name":"Lan","look":{"body":"woman","outfit":"bikini","hat":"none"}}`,
			check: func(t *testing.T, m ClientMsg) {
				if m.Look.Outfit != "bikini" {
					t.Fatalf("got %+v", m)
				}
			}},
		{name: "hello with token", in: `{"t":"hello","token":"abc_-123","name":"Lan","look":{"body":"woman","outfit":"aodai","hat":"nonla"}}`,
			check: func(t *testing.T, m ClientMsg) {
				if m.Token != "abc_-123" {
					t.Fatalf("token = %q", m.Token)
				}
			}},
		{name: "token too long", in: `{"t":"hello","token":"` + strings.Repeat("a", TokenMax+1) + `","name":"Lan","look":{"body":"woman","outfit":"aodai","hat":"nonla"}}`, wantErr: ErrBadToken},
		{name: "token only counts in hello", in: `{"t":"profile","token":"x","name":"Lan","look":{"body":"woman","outfit":"aodai","hat":"nonla"}}`,
			check: func(t *testing.T, m ClientMsg) {
				if m.Token != "" {
					t.Fatalf("token = %q", m.Token)
				}
			}},
		{name: "hello with archetype", in: `{"t":"hello","arch":"driver","name":"Lan","look":{"body":"woman","outfit":"crab","hat":"helmet"}}`,
			check: func(t *testing.T, m ClientMsg) {
				if m.Arch != "driver" || m.Look.Outfit != "crab" {
					t.Fatalf("got %+v", m)
				}
			}},
		{name: "archetype too long", in: `{"t":"hello","arch":"` + strings.Repeat("a", ArchMax+1) + `","name":"Lan","look":{"body":"woman","outfit":"aodai","hat":"nonla"}}`, wantErr: ErrBadArch},
		{name: "chat defaults to global", in: `{"t":"chat","text":"chào"}`,
			check: func(t *testing.T, m ClientMsg) {
				if m.Ch != ChannelGlobal {
					t.Fatalf("ch = %q", m.Ch)
				}
			}},
		{name: "nearby chat", in: `{"t":"chat","ch":"nearby","text":"chào"}`,
			check: func(t *testing.T, m ClientMsg) {
				if m.Ch != ChannelNearby {
					t.Fatalf("ch = %q", m.Ch)
				}
			}},
		{name: "unknown channel", in: `{"t":"chat","ch":"party","text":"chào"}`, wantErr: ErrBadChannel},
		{name: "hello without look", in: `{"t":"hello","name":"Lan"}`, wantErr: ErrBadLook},
		{name: "unknown outfit", in: `{"t":"profile","name":"a","look":{"body":"man","outfit":"suit","hat":"none"}}`, wantErr: ErrBadLook},
		{name: "state wraps heading", in: `{"t":"state","x":1,"y":0.2,"z":-3,"h":7}`,
			check: func(t *testing.T, m ClientMsg) {
				if math.Abs(m.H-(7-2*math.Pi)) > 1e-9 {
					t.Fatalf("h = %v", m.H)
				}
			}},
		{name: "state out of bounds", in: `{"t":"state","x":1e6,"y":0,"z":0,"h":0}`, wantErr: ErrBadPose},
		{name: "state too high", in: `{"t":"state","x":0,"y":500,"z":0,"h":0}`, wantErr: ErrBadPose},
		{name: "chat is cleaned", in: `{"t":"chat","text":"  xin   chào ​ "}`,
			check: func(t *testing.T, m ClientMsg) {
				if m.Text != "xin chào" {
					t.Fatalf("text = %q", m.Text)
				}
			}},
		{name: "blank chat", in: `{"t":"chat","text":" \n "}`, wantErr: ErrEmptyChat},
		{name: "unknown type", in: `{"t":"teleport"}`, wantErr: ErrUnknownType},
		{name: "state flags and corrections", in: `{"t":"state","x":1,"y":0.2,"z":-3,"h":0,"s":33,"k":4}`,
			check: func(t *testing.T, m ClientMsg) {
				if m.S != 33 || m.K != 4 || EmoteOf(m.S) != 2 || m.S&FlagAir == 0 {
					t.Fatalf("got s=%d k=%d", m.S, m.K)
				}
			}},
		{name: "unknown state flag", in: `{"t":"state","x":1,"y":0.2,"z":-3,"h":0,"s":4}`, wantErr: ErrBadFlags},
		{name: "negative state flags", in: `{"t":"state","x":1,"y":0.2,"z":-3,"h":0,"s":-1}`, wantErr: ErrBadFlags},
		{name: "a respawn keeps k, not flags", in: `{"t":"respawn","x":1,"y":0.2,"z":-3,"h":0,"s":33,"k":4}`,
			check: func(t *testing.T, m ClientMsg) {
				if m.S != 0 || m.K != 4 {
					t.Fatalf("got s=%d k=%d, want 0 and 4", m.S, m.K)
				}
			}},
		{name: "respawn out of bounds", in: `{"t":"respawn","x":1e6,"y":0,"z":0,"h":0}`, wantErr: ErrBadPose},
		{name: "mount", in: `{"t":"mount","v":12}`,
			check: func(t *testing.T, m ClientMsg) {
				if m.V != 12 {
					t.Fatalf("v = %d", m.V)
				}
			}},
		{name: "mount without a vehicle", in: `{"t":"mount"}`, wantErr: ErrBadVehicle},
		{name: "dismount", in: `{"t":"dismount","v":3}`,
			check: func(t *testing.T, m ClientMsg) {
				if m.V != 0 {
					t.Fatalf("v = %d", m.V)
				}
			}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, err := Decode([]byte(c.in))
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Fatalf("err = %v, want %v", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			c.check(t, m)
		})
	}
}

func TestDecodeRejectsMalformedJSON(t *testing.T) {
	if _, err := Decode([]byte(`{"t":`)); err == nil {
		t.Fatal("want an error for truncated JSON")
	}
}

func TestPoseArrayRounds(t *testing.T) {
	got := PoseArray(Pose{X: 1.23456, Y: 0.181, Z: -9.87654, H: 3.14159})
	want := [4]float64{1.23, 0.18, -9.88, 3.142}
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestEncodeSnap(t *testing.T) {
	row := AppendSnapRow(nil, 7, Pose{X: 1.234, Y: 0.181, Z: -5, H: 3.14159}, FlagAir|2<<EmoteShift, 40)
	if string(row) != `[7,1.23,0.18,-5,3.142,33,40]` {
		t.Fatalf("row = %s", row)
	}
	var m struct {
		T   string      `json:"t"`
		At  int64       `json:"at"`
		P   [][]float64 `json:"p"`
		Out []uint32    `json:"out"`
	}
	b := EncodeSnap(1730790000000, [][]byte{row, AppendSnapRow(nil, 8, Pose{}, 0, -5)}, []uint32{3, 4})
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%s: %v", b, err)
	}
	if m.T != TypeSnap || m.At != 1730790000000 || len(m.P) != 2 || len(m.P[0]) != 7 || m.P[1][6] != 0 || len(m.Out) != 2 {
		t.Fatalf("decoded %+v from %s", m, b)
	}
	if b := EncodeSnap(1, nil, nil); string(b) != `{"t":"snap","at":1,"p":[]}` {
		t.Fatalf("empty snap = %s", b)
	}
}

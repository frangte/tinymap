package protocol

import (
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

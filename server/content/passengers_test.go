package content

import (
	"math"
	"strings"
	"testing"
)

func TestPassengersContent(t *testing.T) {
	p, err := LoadPassengers()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Cast) < 6 || len(p.Spots) < 12 {
		t.Errorf("%d passengers and %d spots, want at least 6 and 12", len(p.Cast), len(p.Spots))
	}
	// every spot has a destination in range among the landmarks, so a request can always be made from it
	places, err := Places()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range p.Spots {
		x, z, _ := s.ThreePose()
		var near int
		for _, pl := range places {
			if pl.Kind == Landmark && pl.Pts != nil {
				if d := distance(pl, x, z); d >= p.Rules.TripMin && d <= p.Rules.TripMax {
					near++
				}
			}
		}
		if near == 0 {
			t.Errorf("spot %q has no landmark %.0f–%.0f m away", s.ID, p.Rules.TripMin, p.Rules.TripMax)
		}
	}
}

// distance is how far the three.js point (x, z) is from the place's first point, in metres.
func distance(p Place, x, z float64) float64 {
	dx, dy := x-p.Pts[0][0], -z-p.Pts[0][1]
	return math.Hypot(dx, dy)
}

func TestParsePassengersRejects(t *testing.T) {
	const good = `{"rules": {"idle_seconds": 30, "cooldown_seconds": 40, "real_radius": 150, "pickup_min": 25, "pickup_max": 140, "trip_min": 120,
		"trip_max": 380, "board_reach": 4, "patience_seconds": 150, "daily_trips": 8, "linger_seconds": 8, "rating": {"wait_ok": 40, "wait_bad": 80,
		"detour_ok": 1.6, "detour_bad": 2.4, "pace_ok": 5, "pace_bad": 2.5, "scary_speed": 17, "scary_share": 0.35}},
		"cast": [CAST1, CAST2, CAST3],
		"spots": [SPOT1, SPOT2, SPOT3, SPOT4]}`
	cast := func(id string) string {
		line := `[{"vi": "Chào", "en": "Hi"}]`
		return `{"id": "` + id + `", "name": "Chị", "role": {"vi": "khách", "en": "guest"}, "look": {"body": "woman", "outfit": "thun", "hat": "none"},
			"greet": ` + line + `, "thanks": ` + line + `, "grumble": ` + line + `, "left": ` + line + `}`
	}
	spot := func(id string) string {
		return `{"id": "` + id + `", "place": "cho_ben_thanh", "at": [1, 2], "face": [3, 4]}`
	}
	build := func() string {
		s := good
		for i, v := range []string{"CAST1", "CAST2", "CAST3", "SPOT1", "SPOT2", "SPOT3", "SPOT4"} {
			if i < 3 {
				s = strings.Replace(s, v, cast("p"+string(rune('a'+i))), 1)
			} else {
				s = strings.Replace(s, v, spot("s"+string(rune('a'+i))), 1)
			}
		}
		return s
	}
	places := map[string]bool{"cho_ben_thanh": true}
	if _, err := parsePassengers([]byte(build()), places); err != nil {
		t.Fatalf("a good file: %v", err)
	}
	with := func(old, repl string) string { return strings.Replace(build(), old, repl, 1) }
	cases := []struct {
		name string
		json string
		want string
	}{
		{"unknown field", with(`"rules"`, `"mood": 1, "rules"`), "unknown field"},
		{"no daily cap", with(`"daily_trips": 8`, `"daily_trips": 0`), "daily cap"},
		{"trip range backwards", with(`"trip_max": 380`, `"trip_max": 100`), "distances"},
		{"rating order", with(`"wait_bad": 80`, `"wait_bad": 10`), "wait_ok"},
		{"duplicate passenger", strings.Replace(build(), `"id": "pb"`, `"id": "pa"`, 1), "duplicate"},
		{"unknown look", with(`"outfit": "thun"`, `"outfit": "cape"`), "look"},
		{"no English line", with(`"en": "Hi"`, `"en": ""`), "Vietnamese and English"},
		{"a line too long for chat", with(`"vi": "Chào"`, `"vi": "`+strings.Repeat("a", 141)+`"`), "at most"},
		{"unknown place", with(`"place": "cho_ben_thanh"`, `"place": "nowhere"`), "unknown place"},
		{"face on the spot", with(`"face": [3, 4]`, `"face": [1, 2]`), "face"},
		{"too few spots", strings.Replace(build(), `, `+spot("sg"), ``, 1), "at least"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := parsePassengers([]byte(c.json), places)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to mention %q", err, c.want)
			}
		})
	}
}

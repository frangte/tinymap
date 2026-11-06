package content

import (
	"math"
	"strings"
	"testing"

	"saigon-noir/server/internal/protocol"
)

func TestCastLoads(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.NPCs) < 5 {
		t.Fatalf("the roadmap asks for 5 NPCs, there are %d", len(c.NPCs))
	}
	modes := map[string]bool{}
	for _, n := range c.NPCs {
		modes[n.Mode] = true
		if _, ok := c.Dialogue(n.Dialogue); !ok {
			t.Fatalf("%s speaks %q, which did not load", n.ID, n.Dialogue)
		}
	}
	for _, m := range []string{protocol.ModePrivate, protocol.ModePublic, protocol.ModeGroup} {
		if !modes[m] {
			t.Errorf("no NPC talks in %s mode", m)
		}
	}
}

// Every NPC stands on open ground of the island the game ships, inside the place it belongs to.
func TestNPCsStandOnTheIsland(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	grid, err := Nav()
	if err != nil {
		t.Fatal(err)
	}
	places := map[string]Place{}
	for _, p := range c.Places {
		places[p.ID] = p
	}
	for _, n := range c.NPCs {
		x, z, _ := n.ThreePose()
		if !grid.Fits(x, z, 0.3) {
			t.Errorf("%s stands at %v, where a body does not fit on the nav grid", n.ID, n.At)
		}
		p := places[n.Place]
		if d := distToPlace(p, n.At[0], n.At[1]); d > p.R {
			t.Errorf("%s stands %.1f m from %s, whose reach is %.1f m", n.ID, d, p.ID, p.R)
		}
	}
}

func distToPlace(p Place, x, y float64) float64 {
	best := 1e9
	for i, a := range p.Pts {
		d := (a[0]-x)*(a[0]-x) + (a[1]-y)*(a[1]-y)
		if i > 0 {
			b := p.Pts[i-1]
			dx, dy := a[0]-b[0], a[1]-b[1]
			if l := dx*dx + dy*dy; l > 0 {
				t := max(0, min(1, ((x-b[0])*dx+(y-b[1])*dy)/l))
				px, py := b[0]+dx*t, b[1]+dy*t
				d = min(d, (px-x)*(px-x)+(py-y)*(py-y))
			}
		}
		best = min(best, d)
	}
	return math.Sqrt(best)
}

func TestCastRejects(t *testing.T) {
	const npc = `{"id": "x", "name": "Cô X", "role": {"vi": "bán hàng"}, "look": {"body": "woman", "outfit": "baba", "hat": "nonla"},
		"place": "cho_ben_thanh", "at": [2, -11.5], "face": [8, -17], "mode": "private", "dialogue": "x", "reach": 3 %s}`
	npcs := func(extra string) []byte {
		return []byte(`{"npcs": [` + strings.Replace(npc, "%s", extra, 1) + `]}`)
	}
	dialogue := func(nodes string) map[string][]byte {
		return map[string][]byte{"x.json": []byte(`{"id": "x", "start": "a", "nodes": {` + nodes + `}}`)}
	}
	const okNodes = `"a": {"say": {"default": {"vi": "Chào"}}, "choices": [{"text": {"vi": "Thôi"}, "goto": "end"}]}`
	cases := []struct {
		name      string
		npcs      []byte
		dialogues map[string][]byte
		want      string
	}{
		{"unknown mode", npcs(`, "mode": "shout"`), dialogue(okNodes), "mode"},
		{"a dialogue nobody has", npcs(`, "dialogue": "nope"`), dialogue(okNodes), "dialogue \"nope\""},
		{"a look that does not exist", npcs(`, "look": {"body": "cat", "outfit": "baba", "hat": "nonla"}`), dialogue(okNodes), "outfit"},
		{"an arm's length of reach", npcs(`, "reach": 0.5`), dialogue(okNodes), "reach"},
		{"a place that does not exist", npcs(`, "place": "nowhere"`), dialogue(okNodes), "place"},
		{"a bark too long for chat", npcs(`, "barks": [{"vi": "` + strings.Repeat("a", BarkMax+1) + `"}]`), dialogue(okNodes), "bark"},
		{"no start", npcs(""), map[string][]byte{"x.json": []byte(`{"id": "x", "start": "b", "nodes": {` + okNodes + `}}`)}, "start node"},
		{"a file named for another id", npcs(""), map[string][]byte{"y.json": []byte(`{"id": "x", "start": "a", "nodes": {` + okNodes + `}}`)}, "file name"},
		{"a goto to nowhere", npcs(""), dialogue(`"a": {"say": {"default": {"vi": "Chào"}}, "choices": [{"text": {"vi": "Đi"}, "goto": "b"}]}`), "not a node"},
		{"an unreachable node", npcs(""), dialogue(okNodes + `, "b": {"say": {"default": {"vi": "Ơ"}}}`), "cannot be reached"},
		{"no default line", npcs(""), dialogue(`"a": {"say": {"traveler": {"vi": "Chào"}}}`), "default"},
		{"a line for an archetype that does not exist", npcs(""), dialogue(`"a": {"say": {"default": {"vi": "Chào"}, "tourist": {"vi": "Hi"}}}`), "not an archetype"},
		{"a placeholder other than {name}", npcs(""), dialogue(`"a": {"say": {"default": {"vi": "Chào {player}"}}}`), "placeholder"},
		{"an emote that does not exist", npcs(""), dialogue(`"a": {"say": {"default": {"vi": "Chào"}}, "emote": "dance"}`), "emote"},
		{"an answer needing an unknown capability", npcs(""), dialogue(`"a": {"say": {"default": {"vi": "Chào"}}, "choices": [{"text": {"vi": "Đi"}, "goto": "end", "requires": ["no.such"]}]}`), "unknown capability"},
		{"a signal that is not an id", npcs(""), dialogue(`"a": {"say": {"default": {"vi": "Chào"}}, "choices": [{"text": {"vi": "Đi"}, "goto": "end", "emit": "Bought It"}]}`), "signal"},
		{"a public line too long for chat", npcs(`, "mode": "public"`), dialogue(`"a": {"say": {"default": {"vi": "` + strings.Repeat("a", BarkMax+1) + `"}}}`), "public"},
		{"a control character in a line", npcs(""), dialogue(`"a": {"say": {"default": {"vi": "Chào\u0007"}}}`), "control"},
		{"an invisible character in a line", npcs(""), dialogue(`"a": {"say": {"default": {"vi": "Chào\u200b cô"}}}`), "invisible"},
		{"a double space", npcs(""), dialogue(`"a": {"say": {"default": {"vi": "Chào  cô"}}}`), "double"},
		{"a dialogue nobody speaks", npcs(""), map[string][]byte{"x.json": dialogue(okNodes)["x.json"], "y.json": []byte(`{"id": "y", "start": "a", "nodes": {` + okNodes + `}}`)}, "no npc speaks it"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := embedded()
			f.npcs, f.dialogues = c.npcs, c.dialogues
			_, err := load(f)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to mention %q", err, c.want)
			}
		})
	}
	// and the base case loads, so each failure above is the one edit
	f := embedded()
	f.npcs, f.dialogues = npcs(""), dialogue(okNodes)
	if _, err := load(f); err != nil {
		t.Fatalf("the base case: %v", err)
	}
}

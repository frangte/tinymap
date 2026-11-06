package dialogue

import (
	"errors"
	"testing"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/protocol"
)

// a small graph of its own, so the tests do not change whenever the real lines are rewritten
func testScript(t *testing.T) *Script {
	t.Helper()
	c, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	c.Dialogues["test"] = content.Dialogue{ID: "test", Start: "hi", Nodes: map[string]content.DialogueNode{
		"hi": {
			Say: map[string]protocol.Text{
				content.DefaultVariant: {VI: "Chào {name}", EN: "Hi {name}"},
				"local":                {VI: "Ê {name}"},
			},
			Emote: "wave",
			Choices: []content.Choice{
				{Text: protocol.Text{VI: "Bí mật"}, Goto: "secret", Requires: []string{"local.secret"}},
				{Text: protocol.Text{VI: "Mua"}, Goto: "bought", Emit: "bought_it"},
				{Text: protocol.Text{VI: "Thôi"}, Goto: content.DialogueEnd},
			},
		},
		"secret": {Say: map[string]protocol.Text{content.DefaultVariant: {VI: "Suỵt"}}},
		"bought": {Say: map[string]protocol.Text{content.DefaultVariant: {VI: "Của con nè"}}},
	}}
	return NewScript(c)
}

func TestLinesFollowTheSpeaker(t *testing.T) {
	s := testScript(t)
	start, err := s.Begin("test")
	if err != nil || start != "hi" {
		t.Fatalf("begin = %q, %v", start, err)
	}
	cases := []struct {
		name    string
		who     Speaker
		say     string
		choices int
	}{
		{"default line, the name filled in", Speaker{Name: "Lan"}, "Chào Lan", 2},
		{"an archetype with its own line", Speaker{Name: "Minh", Arch: "local"}, "Ê Minh", 2},
		{"an archetype without one gets the default", Speaker{Name: "Tú", Arch: "tourist"}, "Chào Tú", 2},
		{"a capability unlocks an answer", Speaker{Name: "Hoa", Caps: capability.Of("local.secret")}, "Chào Hoa", 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l, err := s.Line("test", "hi", c.who)
			if err != nil {
				t.Fatal(err)
			}
			if l.Say.VI != c.say || len(l.Choices) != c.choices {
				t.Fatalf("got %q with %d choices, want %q with %d", l.Say.VI, len(l.Choices), c.say, c.choices)
			}
			if l.Emote == 0 {
				t.Fatal("the wave emote has no code")
			}
		})
	}
	if l, _ := s.Line("test", "hi", Speaker{Name: "Lan"}); l.Say.EN != "Hi Lan" {
		t.Fatalf("english line %q", l.Say.EN)
	}
}

func TestChoicesIndexWhatTheSpeakerWasOffered(t *testing.T) {
	s := testScript(t)
	plain, local := Speaker{Name: "Lan"}, Speaker{Name: "Hoa", Caps: capability.Of("local.secret")}
	cases := []struct {
		name         string
		who          Speaker
		choice       int
		next, signal string
		err          error
	}{
		{"without the capability, 0 is buying", plain, 0, "bought", "bought_it", nil},
		{"with it, 0 is the secret", local, 0, "secret", "", nil},
		{"the last answer ends it", plain, 1, "", "", nil},
		{"past the end", plain, 2, "", "", ErrChoice},
		{"negative", plain, -1, "", "", ErrChoice},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			next, signal, err := s.Choose("test", "hi", c.who, c.choice)
			if !errors.Is(err, c.err) || next != c.next || signal != c.signal {
				t.Fatalf("got (%q, %q, %v), want (%q, %q, %v)", next, signal, err, c.next, c.signal, c.err)
			}
		})
	}
}

func TestUnknownGraphsAndNodes(t *testing.T) {
	s := testScript(t)
	if _, err := s.Begin("nobody"); !errors.Is(err, ErrUnknown) {
		t.Fatalf("begin: %v", err)
	}
	if _, err := s.Line("test", "nowhere", Speaker{}); !errors.Is(err, ErrUnknown) {
		t.Fatalf("line: %v", err)
	}
	if _, _, err := s.Choose("nobody", "hi", Speaker{}, 0); !errors.Is(err, ErrUnknown) {
		t.Fatalf("choose: %v", err)
	}
}

// Every answer of every NPC's conversation leads somewhere real, and from every line some way leads to an end
// (a goodbye line, or an answer that ends it): no conversation traps anyone in a loop.
func TestEveryConversationCanEnd(t *testing.T) {
	c, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	s := NewScript(c)
	who := Speaker{Name: "Lan"}
	for _, n := range c.NPCs {
		d, _ := c.Dialogue(n.Dialogue)
		next := map[string][]string{}
		ends := map[string]bool{}
		for id := range d.Nodes {
			l, err := s.Line(n.Dialogue, id, who)
			if err != nil {
				t.Fatal(err)
			}
			if len(l.Choices) == 0 {
				ends[id] = true
			}
			for i := range l.Choices {
				to, _, err := s.Choose(n.Dialogue, id, who, i)
				if err != nil {
					t.Fatalf("%s/%s choice %d: %v", n.Dialogue, id, i, err)
				}
				if to == "" {
					ends[id] = true
				} else {
					next[id] = append(next[id], to)
				}
			}
		}
		for changed := true; changed; {
			changed = false
			for id, tos := range next {
				for _, to := range tos {
					if ends[to] && !ends[id] {
						ends[id], changed = true, true
					}
				}
			}
		}
		for id := range d.Nodes {
			if !ends[id] {
				t.Errorf("%s/%s: no way from here leads to an end", n.Dialogue, id)
			}
		}
	}
}

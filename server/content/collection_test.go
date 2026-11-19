package content

import (
	"strings"
	"testing"
)

func TestCollectionContent(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	phrases := 0
	for _, e := range c.Collection {
		if e.Kind == CollectionPhrase {
			phrases++
		}
	}
	if phrases < 8 || phrases > 10 {
		t.Fatalf("%d phrases, want 8 to 10", phrases)
	}
}

func TestParseCollectionRejects(t *testing.T) {
	const entry = `{"id": "xin_chao", "kind": "phrase", "title": {"vi": "Xin chào", "en": "Xin chào"},
		"text": {"vi": "Chào", "en": "Hello"}, "hint": {"vi": "Cô Ba", "en": "Cô Ba"},
		"objective": {"type": "dialogue_signal", "params": {"npc": "co_ba", "signal": "greetings_learned"}}}`
	with := func(old, repl string) string { return strings.Replace(entry, old, repl, 1) }
	cases := []struct {
		name string
		json string
		want string
	}{
		{"an id with capitals", with(`"xin_chao"`, `"XinChao"`), "lower_snake_case"},
		{"a duplicate id", entry + `, ` + entry, "duplicate"},
		{"an unknown kind", with(`"phrase"`, `"song"`), "kind"},
		{"a title without English", with(`"title": {"vi": "Xin chào", "en": "Xin chào"}`, `"title": {"vi": "Xin chào", "en": ""}`), "title"},
		{"a text without English", with(`"en": "Hello"`, `"en": ""`), "text"},
		{"a hint without Vietnamese", with(`"hint": {"vi": "Cô Ba"`, `"hint": {"vi": ""`), "hint"},
		{"an objective with an id", with(`"objective": {`, `"objective": {"id": "x", `), "objective has no id"},
		{"an objective at an unknown place", with(`"type": "dialogue_signal", "params": {"npc": "co_ba", "signal": "greetings_learned"}`, `"type": "reach_place", "params": {"place": "x"}, "at": "nowhere"`), "nowhere"},
		{"an unknown field", with(`"hint"`, `"tip": {}, "hint"`), "unknown field"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := embedded()
			f.collection = []byte(`{"entries": [` + c.json + `]}`)
			_, err := load(f)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to mention %q", err, c.want)
			}
		})
	}
}

func TestCollectionAcceptsAnEmptyGroup(t *testing.T) {
	f := embedded()
	f.collection = []byte(`{"about": "", "entries": []}`)
	if _, err := load(f); err != nil {
		t.Fatal(err)
	}
}

package content

import (
	"strings"
	"testing"
)

func TestTipsContent(t *testing.T) {
	tips, err := LoadTips()
	if err != nil {
		t.Fatal(err)
	}
	if len(tips.Tips) < 8 {
		t.Errorf("%d tips, want at least 8", len(tips.Tips))
	}
}

func TestParseTipsRejects(t *testing.T) {
	const tip = `{"id": "cross_street", "title": {"vi": "Qua đường", "en": "Crossing"}, "text": {"vi": "Đi chậm", "en": "Go slowly"},
		"trigger": {"kind": "place", "place": "cho_ben_thanh"}}`
	places := map[string]bool{"cho_ben_thanh": true}
	npcs := map[string]bool{"chu_sau": true}
	with := func(old, repl string) string { return strings.Replace(tip, old, repl, 1) }
	const trig = `"kind": "place", "place": "cho_ben_thanh"`
	cases := []struct {
		name string
		json string
		want string
	}{
		{"id not snake case", with(`"cross_street"`, `"CrossStreet"`), "lower_snake_case"},
		{"duplicate id", tip + `, ` + tip, "same id"},
		{"no English title", with(`"en": "Crossing"`, `"en": ""`), "title"},
		{"no English text", with(`"en": "Go slowly"`, `"en": ""`), "text"},
		{"unknown kind", with(`"kind": "place"`, `"kind": "weather"`), "unknown trigger kind"},
		{"unknown place", with(`"place": "cho_ben_thanh"`, `"place": "nowhere"`), "unknown place"},
		{"unknown npc", with(trig, `"kind": "npc", "npc": "ai_do"`), "unknown npc"},
		{"bad place kind", with(trig, `"kind": "place_kind", "place_kind": "road"`), "place_kind"},
		{"stray parameter", with(`"place": "cho_ben_thanh"`, `"place": "cho_ben_thanh", "npc": "chu_sau"`), "takes no npc"},
		{"parameter on a plain kind", with(trig, `"kind": "fare_paid", "npc": "chu_sau"`), "takes no npc"},
		{"unknown field", with(`"id"`, `"photo": "", "id"`), "unknown field"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := parseTips([]byte(`{"tips": [`+c.json+`]}`), places, npcs)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to mention %q", err, c.want)
			}
		})
	}
	if _, err := parseTips([]byte(`{"tips": [`+tip+`]}`), places, npcs); err != nil {
		t.Fatalf("a good tip: %v", err)
	}
	if _, err := parseTips([]byte(`{"tips": []}`), places, npcs); err == nil {
		t.Fatal("no tips must fail")
	}
}

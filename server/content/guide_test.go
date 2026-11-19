package content

import (
	"strings"
	"testing"
)

func TestGuideContent(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, e := range c.Guide {
		have[e.Place] = true
	}
	// the first pages of the Cẩm nang (docs/design/10-story.md, section 14)
	for _, id := range []string{"cho_ben_thanh", "nha_tho_duc_ba", "buu_dien_thanh_pho", "nha_hat_thanh_pho", "dinh_doc_lap"} {
		if !have[id] {
			t.Errorf("no guide page for %q", id)
		}
	}
}

func TestParseGuideRejects(t *testing.T) {
	const page = `{"place": "cho_ben_thanh", "checked": "2026-11-19", "summary": {"vi": "Chợ", "en": "Market"},
		"facts": {"built": {"vi": "1914", "en": "1914"}}, "history": [{"vi": "Xưa", "en": "Old"}],
		"see": [], "tips": [], "getting_there": [], "food": [], "maps": "Chợ Bến Thành", "sources": ["https://example.org/a"]}`
	with := func(old, repl string) string { return strings.Replace(page, old, repl, 1) }
	cases := []struct {
		name string
		json string
		want string
	}{
		{"unknown place", with(`"cho_ben_thanh"`, `"nowhere"`), "unknown place"},
		{"two pages for a place", page + `, ` + page, "second page"},
		{"bad day", with(`"2026-11-19"`, `"19/11/2026"`), "YYYY-MM-DD"},
		{"no English summary", with(`"en": "Market"`, `"en": ""`), "summary"},
		{"no quick facts", with(`{"built": {"vi": "1914", "en": "1914"}}`, `{}`), "quick facts"},
		{"no history", with(`[{"vi": "Xưa", "en": "Old"}]`, `[]`), "history"},
		{"a tip without English", with(`"tips": []`, `"tips": [{"vi": "Trả giá"}]`), "tips"},
		{"plain http source", with(`https://example.org/a`, `http://example.org/a`), "https"},
		{"unknown field", with(`"maps"`, `"photo": "", "maps"`), "unknown field"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := embedded()
			f.guide = []byte(`{"entries": [` + c.json + `]}`)
			_, err := load(f)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to mention %q", err, c.want)
			}
		})
	}
}

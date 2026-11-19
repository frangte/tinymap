package content

import (
	"slices"
	"strings"
	"testing"
)

func TestShippedContacts(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	// the NPCs that give a number, and Má, who has no body
	for _, id := range []string{"anh_bay", "chu_sau", "chu_tu", "co_ba", "di_nam", "chi_hanh", "ma"} {
		if _, ok := c.Contact(id); !ok {
			t.Errorf("%s is not a contact", id)
		}
	}
	if _, ok := c.NPCByID("ma"); ok {
		t.Error("Má is on the island")
	}
	if voices := c.VoiceContacts(); len(voices) != 1 || voices[0].ID != "ma" {
		t.Errorf("voices %+v, want only Má", voices)
	}
	// a newcomer starts with Má's number; nobody else does
	for tags, want := range map[string][]string{"citizen newcomer": {"ma"}, "citizen native": nil, "traveler": nil, "driver": nil} {
		if got := c.StartContacts(strings.Fields(tags)); !slices.Equal(got, want) {
			t.Errorf("%s start with %v, want %v", tags, got, want)
		}
	}
	// every contact can be called: its graph has a call node that is a real node
	for _, k := range c.Contacts {
		graph := c.ContactDialogue(k.ID)
		node := c.CallNode(graph)
		if _, ok := c.Dialogues[graph].Nodes[node]; node == "" || !ok {
			t.Errorf("%s: graph %q has call node %q", k.ID, graph, node)
		}
	}
	if c.ContactDialogue("nobody") != "" || c.CallNode("nobody") != "" {
		t.Error("an unknown contact has a graph")
	}
}

// Every text a quest sends comes from a contact, and a contact with the text's sender exists for every quest.
func TestEveryTextSenderIsAContact(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	texts := 0
	for _, q := range c.Quests {
		for _, m := range append(slices.Clone(q.Texts.OnStart), q.Texts.OnDone...) {
			texts++
			if _, ok := c.Contact(m.From); !ok {
				t.Errorf("%s: text from %q, who is not a contact", q.ID, m.From)
			}
		}
	}
	if texts == 0 {
		t.Fatal("no quest sends a text")
	}
	// Má's quest pays 100.000đ and the pay is allowed above a daily's cap
	q, ok := c.Quest("story.goi_ve_cho_ma")
	if !ok || q.Rewards.Xu != 100_000 || !slices.Equal(q.RequiresCaps, []string{CapPhone}) || len(q.Requires) != 0 {
		t.Fatalf("the call home: %+v", q)
	}
	if !c.Emits("ma", "ma_gui_tien") {
		t.Fatal("Má's call has no answer that sends ma_gui_tien")
	}
}

func TestChuSauKnowsANewcomerIsHisFriendsChild(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	d := c.Dialogues["chu_sau"]
	greet := d.Nodes[d.Start].Say
	if greet["newcomer"].VI == "" || greet["newcomer"].EN == "" || greet["newcomer"] == greet[DefaultVariant] {
		t.Fatalf("no newcomer greeting: %+v", greet)
	}
	if greet[DefaultVariant].VI == "" || greet["traveler"].VI == "" {
		t.Fatal("his existing lines are gone")
	}
}

func TestContactsAreChecked(t *testing.T) {
	ma := `{"id": "ma", "name": {"vi": "Má", "en": "Mum"}, "role": {"vi": "má", "en": "mum"}, "dialogue": "ma", "start_for": ["newcomer"]}`
	cases := []struct {
		name     string
		contacts string
		mutate   func(f *files)
		want     string
	}{
		{"a duplicate", `[` + ma + `, ` + ma + `]`, nil, "duplicate"},
		{"an NPC's contact with its own name", `[` + ma + `, {"id": "chu_sau", "name": {"vi": "Sáu", "en": "Sau"}}]`, nil, "leave them out"},
		{"a voice with no start", `[{"id": "ma", "name": {"vi": "Má", "en": "Mum"}, "role": {"vi": "má", "en": "mum"}, "dialogue": "ma"}]`, nil, "start_for"},
		{"a voice with no English", `[{"id": "ma", "name": {"vi": "Má"}, "role": {"vi": "má", "en": "mum"}, "dialogue": "ma", "start_for": ["newcomer"]}]`, nil, "Vietnamese and English"},
		{"a dialogue that is not there", `[` + ma + `, {"id": "me", "name": {"vi": "Mẹ", "en": "Mum"}, "role": {"vi": "mẹ", "en": "mum"}, "dialogue": "mẹ", "start_for": ["newcomer"]}]`, nil, "not in dialogue/"},
		{"a tag nobody has", `[{"id": "ma", "name": {"vi": "Má", "en": "Mum"}, "role": {"vi": "má", "en": "mum"}, "dialogue": "ma", "start_for": ["tourist"]}]`, nil, "start_for tag"},
		{"a graph nobody can call", `[{"id": "ma", "name": {"vi": "Má", "en": "Mum"}, "role": {"vi": "má", "en": "mum"}, "dialogue": "ma", "start_for": ["newcomer"]}]`, func(f *files) {
			f.dialogues["ma.json"] = []byte(strings.Replace(string(f.dialogues["ma.json"]), `"call": "call",`, "", 1))
		}, "no call node"},
		{"a call node that does not exist", `[` + ma + `]`, func(f *files) {
			f.dialogues["ma.json"] = []byte(strings.Replace(string(f.dialogues["ma.json"]), `"call": "call",`, `"call": "nowhere",`, 1))
		}, "call node"},
		{"a text from nobody", `[` + ma + `]`, func(f *files) {
			f.quests = []byte(strings.Replace(string(f.quests), `"from": "ma"`, `"from": "nobody"`, 1))
		}, "not in contacts.json"},
		{"a text too long", `[` + ma + `]`, func(f *files) {
			f.quests = []byte(strings.Replace(string(f.quests), `"Má gửi cho con một trăm ngàn rồi nghen.`, `"`+strings.Repeat("a", TextMax)+` Má gửi cho con một trăm ngàn rồi nghen.`, 1))
		}, "texts.on_done"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := embedded()
			f.contacts = []byte(`{"contacts": ` + c.contacts + `}`)
			if c.mutate != nil {
				d := map[string][]byte{}
				for k, v := range f.dialogues {
					d[k] = v
				}
				f.dialogues = d
				c.mutate(&f)
			}
			_, err := load(f)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to mention %q", err, c.want)
			}
		})
	}
}

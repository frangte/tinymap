package content

import (
	"strings"
	"testing"
)

func TestQuestsLoad(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Quests) == 0 || len(c.Achievements) == 0 {
		t.Fatal("no quests or achievements")
	}
	// the Traveler's questline ends in the certificate
	final, ok := c.Quest("traveler.bai_kiem_tra_cuoi")
	if !ok || final.Rewards.Capability != "local.certified" || len(final.Requires) < 4 {
		t.Fatalf("final quest %+v", final)
	}
	// every tag a quest asks for belongs to some archetype, and every archetype is offered something
	for _, a := range c.Archetypes {
		offered := false
		for _, q := range c.Quests {
			for _, tag := range q.TagsAny {
				for _, mine := range a.Tags {
					offered = offered || tag == mine
				}
			}
		}
		if !offered {
			t.Errorf("archetype %s is offered no quest", a.ID)
		}
	}
}

func TestQuestsRejects(t *testing.T) {
	quest := func(fields string) string {
		base := map[string]string{
			"id": `"a.one"`, "version": `1`, "questline": `"line"`, "title": `{"vi": "Một", "en": "Một"}`, "about": `{"vi": "Thử", "en": "Thử"}`,
			"flow": `"sequence"`, "rewards": `{}`,
			"objectives": `[{"id": "go", "type": "reach_place", "params": {"place": "cho_ben_thanh"}, "text": {"vi": "Đi", "en": "Đi"}}]`,
		}
		for _, kv := range strings.Split(fields, ";") {
			if k, v, ok := strings.Cut(kv, "="); ok {
				base[k] = v
			}
		}
		parts := []string{}
		for k, v := range base {
			parts = append(parts, `"`+k+`": `+v)
		}
		return `{` + strings.Join(parts, ", ") + `}`
	}
	file := func(quests ...string) []byte {
		return []byte(`{"titles": [{"id": "t", "name": {"vi": "Danh hiệu", "en": "Danh hiệu"}}], "stamps": [], "achievements": [], "quests": [` +
			strings.Join(quests, ", ") + `]}`)
	}
	cases := []struct {
		name string
		file []byte
		want string
	}{
		{"a tag no archetype has", file(quest(`tags_any=["tourist"]`)), "tag"},
		{"requiring a quest that does not exist", file(quest(`requires=["a.two"]`)), "requires"},
		{"requiring itself", file(quest(`requires=["a.one"]`)), "requires"},
		{"two quests requiring each other", file(quest(`requires=["a.two"]`), quest(`id="a.two";requires=["a.one"]`)), "loop"},
		{"two quests with one id", file(quest(""), quest("")), "duplicate"},
		{"an unknown flow", file(quest(`flow="whenever"`)), "flow"},
		{"no objectives", file(quest(`objectives=[]`)), "objectives"},
		{"two objectives with one id", file(quest(`objectives=[{"id": "go", "type": "reach_place", "params": {}, "text": {"vi": "Đi", "en": "Đi"}}, {"id": "go", "type": "reach_place", "params": {}, "text": {"vi": "Đi", "en": "Đi"}}]`)), "unique"},
		{"pointing at nothing", file(quest(`objectives=[{"id": "go", "type": "reach_place", "params": {}, "text": {"vi": "Đi", "en": "Đi"}, "at": "nowhere"}]`)), "neither a place nor an NPC"},
		{"params that are not an object", file(quest(`objectives=[{"id": "go", "type": "reach_place", "params": [], "text": {"vi": "Đi", "en": "Đi"}}]`)), "params"},
		{"an objective without text", file(quest(`objectives=[{"id": "go", "type": "reach_place", "params": {}}]`)), "text"},
		{"a title reward that does not exist", file(quest(`rewards={"titles": ["nope"]}`)), "reward title"},
		{"a capability reward that does not exist", file(quest(`rewards={"capability": "no.such"}`)), "reward capability"},
		{"a title too long for a name tag", []byte(`{"titles": [{"id": "t", "name": {"vi": "` + strings.Repeat("a", TitleMax+1) + `"}}], "stamps": [], "achievements": [], "quests": []}`), "title"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := embedded()
			// the shipped dialogue refers to the shipped final quest
			f.quests, f.dialogues = c.file, map[string][]byte{}
			f.npcs = []byte(`{"npcs": []}`)
			_, err := load(f)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to mention %q", err, c.want)
			}
		})
	}
	f := embedded()
	f.quests, f.dialogues, f.npcs = file(quest("")), map[string][]byte{}, []byte(`{"npcs": []}`)
	if _, err := load(f); err != nil {
		t.Fatalf("the base case: %v", err)
	}
}

func TestADialogueAnswerCanRequireAQuest(t *testing.T) {
	f := embedded()
	f.quests = noQuests
	_, err := load(f)
	if err == nil || !strings.Contains(err.Error(), "requires quest") {
		t.Fatalf("the shipped dialogue without its quest: err = %v", err)
	}
}

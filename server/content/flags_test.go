package content

import (
	"strings"
	"testing"
)

const flagText = `{"vi": "Một", "en": "One"}`

// branchQuest is a quest with the given outcomes and extra fields (a JSON fragment starting with a comma).
func branchQuest(id, outcomes, extra string) string {
	return `{"id": "` + id + `", "version": 1, "questline": "line", "title": ` + flagText + `, "about": ` + flagText + `, "flow": "sequence",
		"objectives": [{"id": "go", "type": "reach_place", "params": {"place": "cho_ben_thanh"}, "text": ` + flagText + `}],
		"rewards": {}, "outcomes": ` + outcomes + extra + `}`
}

func outcome(id, extra string) string {
	return `{"id": "` + id + `", "type": "reach_place", "params": {"place": "cho_ben_thanh"}, "text": ` + flagText + `, "title": ` + flagText + extra + `}`
}

func questFile(quests ...string) []byte {
	return []byte(`{"titles": [], "stamps": [], "achievements": [], "quests": [` + strings.Join(quests, ", ") + `]}`)
}

func TestOutcomesAndFlagsInQuests(t *testing.T) {
	two := `[` + outcome("a", "") + `, ` + outcome("b", `, "flags": ["knows_a"]`) + `]`
	reader := func(extra string) string {
		return `{"id": "q.next", "version": 1, "questline": "line", "title": ` + flagText + `, "about": ` + flagText + `, "flow": "sequence",
			"objectives": [{"id": "go", "type": "reach_place", "params": {"place": "cho_ben_thanh"}, "text": ` + flagText + `}], "rewards": {}` + extra + `}`
	}
	cases := []struct {
		name string
		file []byte
		want string // "" loads
	}{
		{"outcomes and a quest that reads their flags", questFile(branchQuest("q.one", two, ""),
			reader(`, "requires_flags": ["q.one:a", "knows_a"], "unless_flags": ["q.one:b"]`)), ""},
		{"a quest with only outcomes", questFile(strings.Replace(branchQuest("q.one", two, ""), `"objectives": [{"id": "go", "type": "reach_place", "params": {"place": "cho_ben_thanh"}, "text": `+flagText+`}]`, `"objectives": []`, 1)), ""},
		{"rewards.flags read elsewhere", questFile(strings.Replace(branchQuest("q.one", two, ""), `"rewards": {}`, `"rewards": {"flags": ["done_q"]}`, 1), reader(`, "requires_flags": ["done_q"]`)), ""},
		{"one outcome", questFile(branchQuest("q.one", `[`+outcome("a", "")+`]`, "")), "needs 2"},
		{"an outcome id that is an objective id", questFile(branchQuest("q.one", `[`+outcome("go", "")+`, `+outcome("b", "")+`]`, "")), "unique"},
		{"two outcomes with one id", questFile(branchQuest("q.one", `[`+outcome("a", "")+`, `+outcome("a", "")+`]`, "")), "unique"},
		{"an outcome without a title", questFile(branchQuest("q.one", `[`+outcome("a", "")+`, {"id": "b", "type": "reach_place", "params": {}, "text": `+flagText+`}]`, "")), "title"},
		{"an outcome pointing nowhere", questFile(branchQuest("q.one", `[`+outcome("a", "")+`, `+outcome("b", `, "at": "nowhere"`)+`]`, "")), "neither a place nor an NPC"},
		{"an outcome with a bad flag", questFile(branchQuest("q.one", `[`+outcome("a", `, "flags": ["Bad Flag"]`)+`, `+outcome("b", "")+`]`, "")), "flag"},
		{"the outcome form set by hand", questFile(branchQuest("q.one", `[`+outcome("a", `, "flags": ["q.one:b"]`)+`, `+outcome("b", "")+`]`, "")), "flag"},
		{"flags in an outcome's rewards", questFile(branchQuest("q.one", `[`+outcome("a", `, "rewards": {"flags": ["x"]}`)+`, `+outcome("b", "")+`]`, "")), "its own flags"},
		{"outcome xu over the story cap with the quest's", questFile(strings.Replace(branchQuest("q.one", `[`+outcome("a", `, "rewards": {"xu": 150000}`)+`, `+outcome("b", "")+`]`, ""), `"rewards": {}`, `"rewards": {"xu": 100000}`, 1)), "together"},
		{"outcomes on a daily", questFile(branchQuest("q.one", two, `, "repeat": "daily"`)), "daily has no outcomes"},
		{"requires_flags on a daily", questFile(strings.Replace(reader(`, "repeat": "daily", "requires_flags": ["q.one:a"]`), "q.next", "q.d", 1), branchQuest("q.one", two, "")), "daily has no outcomes"},
		{"a flag nobody sets", questFile(reader(`, "requires_flags": ["never_set"]`)), "no quest sets it"},
		{"an outcome flag of an outcome that does not exist", questFile(branchQuest("q.one", two, ""), reader(`, "unless_flags": ["q.one:c"]`)), "no quest sets it"},
		{"a flag that is not an id", questFile(reader(`, "requires_flags": ["Not An Id"]`)), "not a flag id"},
		{"a flag only the quest itself sets", questFile(branchQuest("q.one", two, `, "requires_flags": ["knows_a"]`)), "only set by this quest"},
		{"a flag both required and forbidden", questFile(branchQuest("q.one", two, ""), reader(`, "requires_flags": ["knows_a"], "unless_flags": ["knows_a"]`)), "never be offered"},
		{"an achievement that sets flags", []byte(`{"titles": [], "stamps": [], "quests": [], "achievements": [{"id": "ach", "title": ` + flagText + `, "about": ` + flagText +
			`, "objective": {"type": "reach_place", "params": {}}, "rewards": {"flags": ["x"]}}]}`), "sets no flags"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := embedded()
			f.quests, f.dialogues = c.file, map[string][]byte{}
			f.npcs, f.items, f.contacts, f.driving = []byte(`{"npcs": []}`), noItems, noContacts, noDriving
			_, err := load(f)
			switch {
			case c.want == "" && err != nil:
				t.Fatalf("err = %v", err)
			case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
				t.Fatalf("err = %v, want it to mention %q", err, c.want)
			}
		})
	}
}

func TestDialogueReadsFlags(t *testing.T) {
	const npc = `{"npcs": [{"id": "x", "name": "Cô X", "role": {"vi": "bán hàng", "en": "bán hàng"}, "look": {"body": "woman", "outfit": "baba", "hat": "nonla"},
		"place": "cho_ben_thanh", "at": [2, -11.5], "face": [8, -17], "mode": "private", "dialogue": "x", "reach": 3}]}`
	quests := questFile(branchQuest("q.one", `[`+outcome("a", `, "flags": ["knows_a"]`)+`, `+outcome("b", "")+`]`, ""))
	node := func(say, choiceExtra string) map[string][]byte {
		return map[string][]byte{"x.json": []byte(`{"id": "x", "start": "a", "nodes": {"a": {"say": {"default": {"vi": "Chào", "en": "Hi"}` + say + `},
			"choices": [{"text": {"vi": "Đi", "en": "Go"}, "goto": "end"` + choiceExtra + `}]}}}`)}
	}
	line := func(flag string) string { return `, "flag:` + flag + `": {"vi": "Lại là anh", "en": "You again"}` }
	cases := []struct {
		name      string
		dialogues map[string][]byte
		want      string
	}{
		{"a line and an answer for flags that are set", node(line("knows_a")+line("q.one:b"), `, "if_flags": ["q.one:a"], "unless_flags": ["knows_a"]`), ""},
		{"a line for a flag nobody sets", node(line("never_set"), ""), "no quest sets it"},
		{"an answer needing a flag nobody sets", node("", `, "if_flags": ["never_set"]`), "no quest sets it"},
		{"an answer forbidden by a flag nobody sets", node("", `, "unless_flags": ["q.one:c"]`), "no quest sets it"},
		{"an answer both needing and forbidding a flag", node("", `, "if_flags": ["knows_a"], "unless_flags": ["knows_a"]`), "never be offered"},
		{"a line that is no flag, archetype or capability", node(`, "nonsense": {"vi": "Ơ", "en": "Eh"}`, ""), "neither an archetype nor a capability"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := embedded()
			f.npcs, f.dialogues, f.quests, f.items, f.driving, f.contacts = []byte(npc), c.dialogues, quests, noItems, noDriving, noContacts
			_, err := load(f)
			switch {
			case c.want == "" && err != nil:
				t.Fatalf("err = %v", err)
			case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
				t.Fatalf("err = %v, want it to mention %q", err, c.want)
			}
		})
	}
}

func TestFlagSetters(t *testing.T) {
	c := &Catalog{Quests: []Quest{
		{ID: "q.one", Rewards: Rewards{Flags: []string{"f"}}, Outcomes: []QuestOutcome{{Objective: Objective{ID: "a"}, Flags: []string{"f", "g"}}, {Objective: Objective{ID: "b"}}}},
		{ID: "q.two", Rewards: Rewards{Flags: []string{"f"}}},
	}}
	got := c.FlagSetters()
	want := map[string]int{"f": 2, "g": 1, "q.one:a": 1, "q.one:b": 1}
	if len(got) != len(want) {
		t.Fatalf("setters %v", got)
	}
	for f, n := range want {
		if len(got[f]) != n {
			t.Errorf("flag %s set by %v, want %d quests", f, got[f], n)
		}
	}
}

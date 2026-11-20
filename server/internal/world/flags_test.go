package world

import (
	"maps"
	"testing"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/dialogue"
	"saigon-noir/server/internal/protocol"
)

// A flag set while a conversation is open (the answer that set it was rendered before the quest service heard of it)
// makes the conversation show its line again, as the flag changes it.
func TestAFlagChangesTheLineOfTheConversationThatSetIt(t *testing.T) {
	c, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	d := c.Dialogues["co_ba"]
	start := d.Nodes[d.Start]
	say := maps.Clone(start.Say)
	say["flag:fork_done"] = protocol.Text{VI: "Cháu ngoan lắm.", EN: "Good kid."}
	start.Say = say
	d.Nodes[d.Start] = start
	h := newHarnessWith(t, func(cfg *Config) { cfg.NPCs, cfg.Dialogue = NPCsFrom(c), dialogue.NewScript(c) })
	coBa := h.npc("co_ba")
	p, m := h.joinNear("Lan", "traveler", coBa, 1.5, 0)
	h.interact(p, coBa, 1)
	fs, _ := h.drain(p)
	before := only(t, fs, protocol.TypeInteraction)
	if before.Say.EN == "Good kid." {
		t.Fatal("the flag line is shown to someone without the flag")
	}

	// the same flags again change nothing: no new line
	h.w.onService(event{kind: evFlags, character: "char-Lan", flags: nil})
	if fs, _ := h.drain(p); len(fs) != 0 {
		t.Fatalf("frames after an unchanged flag set: %+v", fs)
	}

	h.w.onService(event{kind: evFlags, character: "char-Lan", flags: map[string]bool{"fork_done": true}})
	fs, _ = h.drain(p)
	after := only(t, fs, protocol.TypeInteraction)
	if after.Say.EN != "Good kid." || after.Sid != before.Sid || after.Step != before.Step {
		t.Fatalf("after the flag: %+v (before %+v)", after, before)
	}
	if !m.story["fork_done"] {
		t.Fatal("the member does not hold the flag")
	}
}

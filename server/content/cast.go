package content

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"saigon-noir/server/internal/protocol"
)

// The cast: the NPCs on the island (npcs.json, shared with the game, which draws them) and what they say
// (dialogue/*.json, server only: the server picks every line, so the game never has the whole script).

//go:embed npcs.json
var npcsJSON []byte

//go:embed dialogue/*.json
var dialogueFS embed.FS

func dialogueFiles() map[string][]byte {
	out := map[string][]byte{}
	entries, err := fs.ReadDir(dialogueFS, "dialogue")
	if err != nil {
		panic("content: dialogue/ is embedded: " + err.Error())
	}
	for _, e := range entries {
		b, err := dialogueFS.ReadFile(path.Join("dialogue", e.Name()))
		if err != nil {
			panic("content: dialogue/ is embedded: " + err.Error())
		}
		out[e.Name()] = b
	}
	return out
}

// NPC is a non-player character: one shared figure that any number of players can talk to at once
// (docs/design/02-architecture.md, "One NPC, many conversations").
type NPC struct {
	ID string `json:"id"`
	// Name is how they are called; it is not translated
	Name string `json:"name"`
	// Role is what they do, under the name
	Role Text          `json:"role"`
	Look protocol.Look `json:"look"`
	// Place is where they belong (places.json); At is where they stand and Face the point they look toward when
	// nobody is talking to them, in Blender XY
	Place string    `json:"place"`
	At    []float64 `json:"at"`
	Face  []float64 `json:"face"`
	// Mode is how a conversation with them goes: protocol.ModePrivate, ModePublic or ModeGroup
	Mode string `json:"mode"`
	// Dialogue is the graph they speak (dialogue/<id>.json)
	Dialogue string `json:"dialogue"`
	// Reach is how close a player must stand to start talking (m); walking further than Reach plus a little
	// ends the conversation
	Reach float64 `json:"reach"`
	// Barks are what they call out to passers-by now and then, heard nearby
	Barks []Text `json:"barks,omitempty"`
	// Vehicle is a vehicle kind the game parks beside them, for the look of it (nobody can ride it)
	Vehicle string `json:"vehicle,omitempty"`
}

// ThreePose is where the NPC stands in three.js terms, facing Face.
func (n NPC) ThreePose() (x, z, heading float64) { return threePose(n.At, n.Face) }

// DialogueEnd, as a choice's goto, ends the conversation.
const DialogueEnd = "end"

// DefaultVariant is the line for a speaker whose archetype has no variant of its own.
const DefaultVariant = "default"

// Dialogue is a conversation graph. It moves from node to node by the answers players pick; an answer may send a
// signal (quests listen for them, phase 4). Dialogue never hands out anything itself.
type Dialogue struct {
	ID    string                  `json:"id"`
	About string                  `json:"about,omitempty"`
	Start string                  `json:"start"`
	Nodes map[string]DialogueNode `json:"nodes"`
}

// DialogueNode is one line of the NPC and the answers to it. Say has a variant per archetype id, and
// DefaultVariant for everyone else: the archetype changes the tone, not the language (01-gameplay.md).
// "{name}" in a line becomes the player's name. A node without choices ends the conversation: only goodbye is
// left.
type DialogueNode struct {
	Say map[string]Text `json:"say"`
	// Emote is played by the NPC as it says the line (emotes.json), "" for none
	Emote   string   `json:"emote,omitempty"`
	Choices []Choice `json:"choices,omitempty"`
}

// Choice is an answer to a line.
type Choice struct {
	Text Text `json:"text"`
	// Goto is the next node, or DialogueEnd
	Goto string `json:"goto"`
	// Emit is the signal picking it sends (quests listen for these), "" for none
	Emit string `json:"emit,omitempty"`
	// Requires: only players with every one of these capabilities are offered it
	Requires []string `json:"requires,omitempty"`
	// RequiresQuest: only players with this quest running (quests.json) are offered it
	RequiresQuest string `json:"requires_quest,omitempty"`
}

// Limits on what NPCs say (characters).
const (
	SayMax     = 320
	ChoiceMax  = 90
	ChoicesMax = 6
	BarkMax    = protocol.ChatMax
)

var (
	signalPattern = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*$`)
	// the only placeholder a line may hold
	placeholder = regexp.MustCompile(`\{[^}]*\}`)
)

var modes = map[string]bool{protocol.ModePrivate: true, protocol.ModePublic: true, protocol.ModeGroup: true}

func parseCast(f files, c *Catalog) error {
	var n struct {
		About string `json:"about"`
		NPCs  []NPC  `json:"npcs"`
	}
	if err := decodeStrict(f.npcs, &n); err != nil {
		return fmt.Errorf("npcs.json: %w", err)
	}
	c.NPCs = n.NPCs
	c.Dialogues = map[string]Dialogue{}
	names := make([]string, 0, len(f.dialogues))
	for name := range f.dialogues {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		var d Dialogue
		if err := decodeStrict(f.dialogues[name], &d); err != nil {
			return fmt.Errorf("dialogue/%s: %w", name, err)
		}
		if want := strings.TrimSuffix(name, ".json"); d.ID != want {
			return fmt.Errorf("dialogue/%s: id %q must be the file name, %q", name, d.ID, want)
		}
		c.Dialogues[d.ID] = d
	}
	return nil
}

// checkCast runs after the archetypes, capabilities, places, vehicles and emotes are checked.
func (c *Catalog) checkCast(caps, places map[string]bool) error {
	used := map[string]bool{}
	seen := map[string]bool{}
	for _, n := range c.NPCs {
		if err := c.checkNPC(n, places); err != nil {
			return fmt.Errorf("npcs.json: npc %q: %w", n.ID, err)
		}
		if seen[n.ID] {
			return fmt.Errorf("npcs.json: npc %q: duplicate id", n.ID)
		}
		seen[n.ID] = true
		used[n.Dialogue] = true
	}
	for _, id := range sortedKeys(c.Dialogues) {
		if err := c.checkDialogue(c.Dialogues[id], caps); err != nil {
			return fmt.Errorf("dialogue/%s.json: %w", id, err)
		}
		if !used[id] {
			return fmt.Errorf("dialogue/%s.json: no npc speaks it", id)
		}
	}
	for _, n := range c.NPCs {
		if n.Mode == protocol.ModePublic {
			if err := heardWhole(c.Dialogues[n.Dialogue]); err != nil {
				return fmt.Errorf("dialogue/%s.json: %s talks in public: %w", n.Dialogue, n.ID, err)
			}
		}
	}
	return nil
}

// heardWhole checks that every line of d fits in a chat line: a public NPC's lines are also heard nearby.
func heardWhole(d Dialogue) error {
	for _, id := range sortedKeys(d.Nodes) {
		for k, t := range d.Nodes[id].Say {
			if utf8.RuneCountInString(t.VI) > BarkMax {
				return fmt.Errorf("node %q, say %q is longer than %d characters", id, k, BarkMax)
			}
		}
	}
	return nil
}

func (c *Catalog) checkNPC(n NPC, places map[string]bool) error {
	switch {
	case !idPattern.MatchString(n.ID):
		return errors.New("id must be lower_snake_case ascii")
	case n.Name == "" || protocol.CleanName(n.Name) != n.Name:
		return fmt.Errorf("needs a name of at most %d characters, without extra spaces", protocol.NameMax)
	case n.Role.VI == "" || n.Role.EN == "":
		return errors.New("needs a role in Vietnamese and English")
	case !n.Look.Valid():
		return errors.New("unknown body, outfit or hat")
	case !places[n.Place]:
		return fmt.Errorf("place %q is not in places.json", n.Place)
	case !point(n.At) || !point(n.Face):
		return fmt.Errorf("at and face must be [x, y] within ±%d m", coordLimit)
	case n.At[0] == n.Face[0] && n.At[1] == n.Face[1]:
		return errors.New("face must be another point than at")
	case !modes[n.Mode]:
		return fmt.Errorf("mode must be %s, %s or %s", protocol.ModePrivate, protocol.ModePublic, protocol.ModeGroup)
	case c.Dialogues[n.Dialogue].ID == "":
		return fmt.Errorf("dialogue %q is not in dialogue/", n.Dialogue)
	case !(n.Reach >= 1.5 && n.Reach <= 6):
		return errors.New("reach must be in [1.5, 6] m")
	case n.Vehicle != "" && c.vehicleByID[n.Vehicle].ID == "":
		return fmt.Errorf("vehicle %q is not in vehicles.json", n.Vehicle)
	}
	for _, b := range n.Barks {
		if err := checkText(b, BarkMax, false); err != nil {
			return fmt.Errorf("bark %q: %w", b.VI, err)
		}
	}
	return nil
}

func (c *Catalog) checkDialogue(d Dialogue, caps map[string]bool) error {
	if !idPattern.MatchString(d.ID) {
		return errors.New("id must be lower_snake_case ascii")
	}
	if _, ok := d.Nodes[d.Start]; !ok {
		return fmt.Errorf("start node %q does not exist", d.Start)
	}
	for _, id := range sortedKeys(d.Nodes) {
		if !idPattern.MatchString(id) || id == DialogueEnd {
			return fmt.Errorf("node %q: ids must be lower_snake_case ascii, and not %q", id, DialogueEnd)
		}
		if err := c.checkNode(d, d.Nodes[id], caps); err != nil {
			return fmt.Errorf("node %q: %w", id, err)
		}
	}
	// every node can be reached from the start: an unreachable one is a typo in some goto
	reached := map[string]bool{d.Start: true}
	queue := []string{d.Start}
	for len(queue) > 0 {
		n := d.Nodes[queue[0]]
		queue = queue[1:]
		for _, ch := range n.Choices {
			if ch.Goto != DialogueEnd && !reached[ch.Goto] {
				reached[ch.Goto] = true
				queue = append(queue, ch.Goto)
			}
		}
	}
	for _, id := range sortedKeys(d.Nodes) {
		if !reached[id] {
			return fmt.Errorf("node %q cannot be reached from the start", id)
		}
	}
	return nil
}

func (c *Catalog) checkNode(d Dialogue, n DialogueNode, caps map[string]bool) error {
	if _, ok := n.Say[DefaultVariant]; !ok {
		return fmt.Errorf("say needs a %q line", DefaultVariant)
	}
	for _, k := range sortedKeys(n.Say) {
		if k != DefaultVariant && c.byID[k].ID == "" {
			return fmt.Errorf("say has a line for %q, which is not an archetype", k)
		}
		if err := checkText(n.Say[k], SayMax, true); err != nil {
			return fmt.Errorf("say %q: %w", k, err)
		}
	}
	if _, ok := c.Emote(n.Emote); n.Emote != "" && !ok {
		return fmt.Errorf("emote %q is not in emotes.json", n.Emote)
	}
	if len(n.Choices) > ChoicesMax {
		return fmt.Errorf("at most %d choices", ChoicesMax)
	}
	for i, ch := range n.Choices {
		if err := checkText(ch.Text, ChoiceMax, true); err != nil {
			return fmt.Errorf("choice %d: %w", i, err)
		}
		if _, ok := d.Nodes[ch.Goto]; !ok && ch.Goto != DialogueEnd {
			return fmt.Errorf("choice %d goes to %q, which is not a node (or %q)", i, ch.Goto, DialogueEnd)
		}
		if ch.Emit != "" && !signalPattern.MatchString(ch.Emit) {
			return fmt.Errorf("choice %d: signal %q must be lower_snake_case ascii", i, ch.Emit)
		}
		for _, r := range ch.Requires {
			if !caps[r] {
				return fmt.Errorf("choice %d requires unknown capability %q", i, r)
			}
		}
		if _, ok := c.questByID[ch.RequiresQuest]; ch.RequiresQuest != "" && !ok {
			return fmt.Errorf("choice %d requires quest %q, which is not in quests.json", i, ch.RequiresQuest)
		}
	}
	return nil
}

// checkText checks a line: Vietnamese required, both languages within max characters, clean of control
// characters, and (where allowed) no placeholder but {name}.
func checkText(t Text, max int, placeholders bool) error {
	if t.VI == "" {
		return errors.New("needs Vietnamese")
	}
	// the game shows the English to players who chose it (src/game/i18n), so every line has one
	if t.EN == "" {
		return errors.New("needs English")
	}
	for _, s := range []string{t.VI, t.EN} {
		if utf8.RuneCountInString(s) > max {
			return fmt.Errorf("longer than %d characters", max)
		}
		// what players see is the line as chat cleans it: anything cleaning would change is a mistake here
		if s != "" && protocol.CleanChat(s) != s && utf8.RuneCountInString(s) <= protocol.ChatMax {
			return errors.New("no control or invisible characters, line breaks, double or leading and trailing spaces")
		}
		for _, r := range s {
			if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
				return errors.New("no control or invisible characters")
			}
		}
		if strings.TrimSpace(s) != s || strings.Contains(s, "  ") {
			return errors.New("no double, leading or trailing spaces")
		}
		for _, p := range placeholder.FindAllString(s, -1) {
			if !placeholders || p != "{name}" {
				return fmt.Errorf("unknown placeholder %s (only {name})", p)
			}
		}
	}
	return nil
}

// Emote finds an emote by id.
func (c *Catalog) Emote(id string) (Emote, bool) {
	for _, e := range c.Emotes {
		if e.ID == id {
			return e, true
		}
	}
	return Emote{}, false
}

// Dialogue finds a dialogue graph by id.
func (c *Catalog) Dialogue(id string) (Dialogue, bool) {
	d, ok := c.Dialogues[id]
	return d, ok
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

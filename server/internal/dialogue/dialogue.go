// Package dialogue runs NPC conversations. It only knows graphs, nodes and answers: who is talking to whom, how
// close they stand and who leads a group is the world's business (package world). Keeping it apart is what lets
// an AI-written provider replace the scripted one later without touching NPC behaviour
// (docs/design/04-backend.md, "Future AI NPC dialogue"); like Script, it may only move along the graph's edges
// and send the signals the graph lists, so it can never grant anything itself.
package dialogue

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/protocol"
)

// Speaker is who a line is for: the variant of a line follows the archetype, the answers offered follow the
// capabilities and the quests running, {name} in a line becomes Name and {anh_chi} (Vietnamese) follows Body.
type Speaker struct {
	Name string
	Arch string
	// Body is the look's body, "man" or "woman" (protocol.Look): anything else is taken as a man's
	Body string
	Caps capability.Set
	// Quests are the ids of the quests the speaker has running
	Quests map[string]bool
	// Flags are the story flags the speaker holds (set by quests, never by this package): answers and lines can
	// depend on them
	Flags map[string]bool
	// Marks are the conversation's tallies (a quiz's wrong answers); Choose adds to them and a node may reset
	// them. The world keeps the map with the conversation; nil counts nothing.
	Marks map[string]int
}

// Line is a node of a conversation as one speaker sees it.
type Line struct {
	Say protocol.Text
	// Emote is the code of the emote the NPC plays as it speaks (emotes.json), 0 for none, for EmoteFor
	Emote    int
	EmoteFor time.Duration
	// Shop is the shop (items.json) this line opens, "" for none
	Shop string
	// Fee is the fee (items.json) this line asks for, "" for none
	Fee string
	// Choices are the answers the speaker is offered, by index; none means only goodbye is left
	Choices []protocol.Text
}

// Provider runs conversations.
type Provider interface {
	// Begin is the node a conversation with this graph starts at.
	Begin(graph string) (node string, err error)
	// Call is the node a phone call to this graph starts at; false for a graph nobody can call.
	Call(graph string) (node string, ok bool)
	// Line renders a node for a speaker.
	Line(graph, node string, who Speaker) (Line, error)
	// Choose applies the speaker's answer (an index into Line's choices for them).
	Choose(graph, node string, who Speaker, choice int) (Step, error)
}

// Step is where an answer leads: Next is the node, "" when the conversation ends; Signals are what the answer and
// the node it arrives at send, in that order.
type Step struct {
	Next    string
	Signals []string
}

var (
	// ErrUnknown: no such graph or node (content changed under a running conversation, or a bug)
	ErrUnknown = errors.New("dialogue: unknown graph or node")
	// ErrChoice: the speaker was not offered that answer
	ErrChoice = errors.New("dialogue: no such choice")
)

// loopFor is how long an NPC plays an emote that loops (emotes.json seconds 0).
const loopFor = 4 * time.Second

// Script is the scripted provider: the graphs in server/content/dialogue.
type Script struct {
	catalog *content.Catalog
}

func NewScript(c *content.Catalog) *Script { return &Script{catalog: c} }

func (s *Script) node(graph, node string) (content.DialogueNode, error) {
	d, ok := s.catalog.Dialogue(graph)
	if !ok {
		return content.DialogueNode{}, fmt.Errorf("%w: %q", ErrUnknown, graph)
	}
	n, ok := d.Nodes[node]
	if !ok {
		return content.DialogueNode{}, fmt.Errorf("%w: %q in %q", ErrUnknown, node, graph)
	}
	return n, nil
}

func (s *Script) Begin(graph string) (string, error) {
	d, ok := s.catalog.Dialogue(graph)
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknown, graph)
	}
	return d.Start, nil
}

func (s *Script) Call(graph string) (string, bool) {
	node := s.catalog.CallNode(graph)
	return node, node != ""
}

func (s *Script) Line(graph, node string, who Speaker) (Line, error) {
	n, err := s.node(graph, node)
	if err != nil {
		return Line{}, err
	}
	say := variant(n, who)
	l := Line{Say: fill(say, who), Shop: n.Shop, Fee: n.Fee, Choices: []protocol.Text{}}
	if e, ok := s.catalog.Emote(n.Emote); ok {
		l.Emote, l.EmoteFor = e.Code, time.Duration(e.Seconds*float64(time.Second))
		if l.EmoteFor <= 0 {
			l.EmoteFor = loopFor // an emote that loops for players plays a while for an NPC
		}
	}
	for _, ch := range offered(n, who) {
		l.Choices = append(l.Choices, fill(ch.Text, who))
	}
	return l, nil
}

func (s *Script) Choose(graph, node string, who Speaker, choice int) (Step, error) {
	n, err := s.node(graph, node)
	if err != nil {
		return Step{}, err
	}
	on := offered(n, who)
	if choice < 0 || choice >= len(on) {
		return Step{}, ErrChoice
	}
	ch := on[choice]
	var st Step
	if ch.Emit != "" {
		st.Signals = append(st.Signals, ch.Emit)
	}
	if ch.Mark != "" && who.Marks != nil {
		who.Marks[ch.Mark]++
	}
	next := ch.Goto
	if g := ch.GotoIf; g != nil && who.Marks[g.Mark] >= g.AtLeast {
		next = g.Goto
	}
	if next == content.DialogueEnd {
		return st, nil
	}
	st.Next = next
	arrived, err := s.node(graph, next)
	if err != nil {
		return Step{}, err
	}
	if arrived.ResetMarks {
		clear(who.Marks)
	}
	if arrived.Emit != "" {
		st.Signals = append(st.Signals, arrived.Emit)
	}
	return st, nil
}

// offered is the answers of n the speaker may pick, in order.
func offered(n content.DialogueNode, who Speaker) []content.Choice {
	out := make([]content.Choice, 0, len(n.Choices))
	for _, ch := range n.Choices {
		if who.Caps.HasAll(ch.Requires) && !hasAny(who.Caps, ch.Unless) && (ch.RequiresQuest == "" || who.Quests[ch.RequiresQuest]) &&
			holdsAll(who.Flags, ch.IfFlags) && !holdsAny(who.Flags, ch.UnlessFlags) {
			out = append(out, ch)
		}
	}
	return out
}

// fill puts the speaker into a line. {anh_chi} is only for Vietnamese (content validation keeps it out of English
// lines), where a peer says "anh" to a man and "chị" to a woman.
func fill(t protocol.Text, who Speaker) protocol.Text {
	anhChi := "anh"
	if who.Body == "woman" {
		anhChi = "chị"
	}
	vi := strings.NewReplacer(content.PlaceholderName, who.Name, content.PlaceholderAnhChi, anhChi).Replace(t.VI)
	return protocol.Text{VI: vi, EN: strings.ReplaceAll(t.EN, content.PlaceholderName, who.Name)}
}

// variant is the line for the speaker: the one for a story flag it holds (a key "flag:<id>"; the first in
// alphabetical order), else the one for its archetype, else the one for a capability it has (a key
// with a dot, such as "ridehail.drive"; the first in alphabetical order), else the default.
func variant(n content.DialogueNode, who Speaker) protocol.Text {
	keys := make([]string, 0, len(n.Say))
	for k := range n.Say {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if flag, ok := strings.CutPrefix(k, content.SayFlagPrefix); ok && who.Flags[flag] {
			return n.Say[k]
		}
	}
	if say, ok := n.Say[who.Arch]; ok {
		return say
	}
	for _, k := range keys {
		if who.Caps.Has(k) {
			return n.Say[k]
		}
	}
	return n.Say[content.DefaultVariant]
}

func holdsAll(flags map[string]bool, ids []string) bool {
	for _, id := range ids {
		if !flags[id] {
			return false
		}
	}
	return true
}

func holdsAny(flags map[string]bool, ids []string) bool {
	for _, id := range ids {
		if flags[id] {
			return true
		}
	}
	return false
}

func hasAny(caps capability.Set, ids []string) bool {
	for _, id := range ids {
		if caps.Has(id) {
			return true
		}
	}
	return false
}

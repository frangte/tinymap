// Package dialogue runs NPC conversations. It only knows graphs, nodes and answers: who is talking to whom, how
// close they stand and who leads a group is the world's business (package world). Keeping it apart is what lets
// an AI-written provider replace the scripted one later without touching NPC behaviour
// (docs/design/04-backend.md, "Future AI NPC dialogue"); like Script, it may only move along the graph's edges
// and send the signals the graph lists, so it can never grant anything itself.
package dialogue

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/protocol"
)

// Speaker is who a line is for: the variant of a line follows the archetype, the answers offered follow the
// capabilities and the quests running, and {name} in a line becomes Name.
type Speaker struct {
	Name string
	Arch string
	Caps capability.Set
	// Quests are the ids of the quests the speaker has running
	Quests map[string]bool
}

// Line is a node of a conversation as one speaker sees it.
type Line struct {
	Say protocol.Text
	// Emote is the code of the emote the NPC plays as it speaks (emotes.json), 0 for none, for EmoteFor
	Emote    int
	EmoteFor time.Duration
	// Choices are the answers the speaker is offered, by index; none means only goodbye is left
	Choices []protocol.Text
}

// Provider runs conversations.
type Provider interface {
	// Begin is the node a conversation with this graph starts at.
	Begin(graph string) (node string, err error)
	// Line renders a node for a speaker.
	Line(graph, node string, who Speaker) (Line, error)
	// Choose applies the speaker's answer (an index into Line's choices for them): next is the node it leads to,
	// "" when the conversation ends; signal is what the answer sends, "" for nothing.
	Choose(graph, node string, who Speaker, choice int) (next, signal string, err error)
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

func (s *Script) Line(graph, node string, who Speaker) (Line, error) {
	n, err := s.node(graph, node)
	if err != nil {
		return Line{}, err
	}
	say, ok := n.Say[who.Arch]
	if !ok {
		say = n.Say[content.DefaultVariant]
	}
	l := Line{Say: fill(say, who.Name), Choices: []protocol.Text{}}
	if e, ok := s.catalog.Emote(n.Emote); ok {
		l.Emote, l.EmoteFor = e.Code, time.Duration(e.Seconds*float64(time.Second))
		if l.EmoteFor <= 0 {
			l.EmoteFor = loopFor // an emote that loops for players plays a while for an NPC
		}
	}
	for _, ch := range offered(n, who) {
		l.Choices = append(l.Choices, fill(ch.Text, who.Name))
	}
	return l, nil
}

func (s *Script) Choose(graph, node string, who Speaker, choice int) (string, string, error) {
	n, err := s.node(graph, node)
	if err != nil {
		return "", "", err
	}
	on := offered(n, who)
	if choice < 0 || choice >= len(on) {
		return "", "", ErrChoice
	}
	ch := on[choice]
	if ch.Goto == content.DialogueEnd {
		return "", ch.Emit, nil
	}
	return ch.Goto, ch.Emit, nil
}

// offered is the answers of n the speaker may pick, in order.
func offered(n content.DialogueNode, who Speaker) []content.Choice {
	out := make([]content.Choice, 0, len(n.Choices))
	for _, ch := range n.Choices {
		if who.Caps.HasAll(ch.Requires) && (ch.RequiresQuest == "" || who.Quests[ch.RequiresQuest]) {
			out = append(out, ch)
		}
	}
	return out
}

func fill(t protocol.Text, name string) protocol.Text {
	return protocol.Text{VI: strings.ReplaceAll(t.VI, "{name}", name), EN: strings.ReplaceAll(t.EN, "{name}", name)}
}

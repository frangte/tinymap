package world

import (
	"errors"
	"slices"

	"saigon-noir/server/internal/dialogue"
	"saigon-noir/server/internal/protocol"
)

// A conversation (docs/design/02-architecture.md, "One NPC, many conversations"). The NPC is shared; each
// conversation keeps its own place in the dialogue. Private and public conversations have one player each (a
// public NPC's lines are also heard nearby); a group NPC has one conversation that everyone who walks up joins,
// led by whoever has been in it longest. A player is in one conversation at a time, and leaves it by closing it,
// by walking away, or by starting another.
type session struct {
	id   uint32
	npc  *npc
	mode string
	// members are its players, the leader (who picks the answers) first
	members []*member
	// node is where the conversation is in the NPC's dialogue graph; step counts the lines it has shown
	node string
	step uint32
}

// Signal is what an answer sends (server/content/dialogue): quests listen for these.
type Signal struct {
	// Character is the stored character, "" for a guest without an account; Player its id in the world
	Character string
	Player    uint32
	// NPC is the NPC's content id
	NPC    string
	Signal string
}

func (m *member) speaker() dialogue.Speaker {
	return dialogue.Speaker{Name: m.name, Arch: m.arch, Caps: m.caps}
}

// onInteract starts a conversation with an NPC, or joins its group one.
func (w *World) onInteract(p *Peer, m *member, msg protocol.ClientMsg) {
	n := w.npcByID[msg.N]
	switch {
	case n == nil:
		w.result(p, msg.Rid, protocol.ResTarget)
		return
	case w.cfg.Dialogue == nil:
		w.result(p, msg.Rid, protocol.ResMute)
		return
	case n.dist2(m) > n.spec.Reach*n.spec.Reach:
		w.result(p, msg.Rid, protocol.ResFar)
		return
	}
	if s := m.talk; s != nil {
		if s.npc == n {
			// already talking to it (a press while the answer was on its way): the line again
			w.show(s, m)
			w.result(p, msg.Rid, "")
			return
		}
		w.leaveTalk(m, protocol.EndOther)
		// telling it may have found its queue full and dropped it: nothing more to do for a player who left
		if w.members[p] != m {
			return
		}
	}
	if s := n.group; s != nil {
		if len(s.members) >= w.cfg.Talk.MaxGroup {
			w.result(p, msg.Rid, protocol.ResFull)
			return
		}
		s.members = append(s.members, m)
		m.talk = s
		n.turnTo(m)
		w.showAll(s)
		w.result(p, msg.Rid, "")
		return
	}
	node, err := w.cfg.Dialogue.Begin(n.spec.Dialogue)
	if err != nil {
		w.log.Error("npc cannot talk", "npc", n.spec.Key, "err", err)
		w.result(p, msg.Rid, protocol.ResMute)
		return
	}
	w.nextSession++
	s := &session{id: w.nextSession, npc: n, mode: n.spec.Mode, members: []*member{m}, node: node}
	w.sessions[s.id] = s
	m.talk = s
	n.talks++
	if s.mode == protocol.ModeGroup {
		n.group = s
	}
	if !w.speak(s, m) {
		w.result(p, msg.Rid, protocol.ResMute)
		return
	}
	w.result(p, msg.Rid, "")
}

// onChoose applies the leader's answer: the conversation moves on, or ends.
func (w *World) onChoose(p *Peer, m *member, msg protocol.ClientMsg) {
	s := m.talk
	switch {
	case s == nil || s.id != msg.Sid:
		w.result(p, msg.Rid, protocol.ResStale)
		return
	case s.members[0] != m:
		w.result(p, msg.Rid, protocol.ResLead)
		return
	case msg.Step != s.step:
		// an answer to a line the conversation has moved past (a click that crossed the next line on the wire)
		w.result(p, msg.Rid, protocol.ResStale)
		return
	}
	n := s.npc
	next, signal, err := w.cfg.Dialogue.Choose(n.spec.Dialogue, s.node, m.speaker(), msg.C)
	if errors.Is(err, dialogue.ErrChoice) {
		w.result(p, msg.Rid, protocol.ResChoice)
		return
	}
	if err != nil {
		w.log.Error("conversation broke", "npc", n.spec.Key, "node", s.node, "err", err)
		w.endTalk(s, protocol.EndDone)
		w.result(p, msg.Rid, protocol.ResStale)
		return
	}
	if signal != "" {
		// everyone in the conversation heard the answer
		for _, o := range s.members {
			w.signal(o, n, signal)
		}
	}
	if next == "" {
		w.endTalk(s, protocol.EndDone)
		w.result(p, msg.Rid, "")
		return
	}
	s.node = next
	if !w.speak(s, m) {
		w.result(p, msg.Rid, protocol.ResStale)
		return
	}
	w.result(p, msg.Rid, "")
}

// speak shows the conversation's line to its players and makes the NPC say it: it turns toward `by`, who spoke
// last, plays the line's emote, and (in public) is heard nearby, at most once per PublicEvery so that answering
// fast cannot fill the chat of everyone around. False when the line could not be rendered (the conversation
// ended).
func (w *World) speak(s *session, by *member) bool {
	n := s.npc
	line, ok := w.line(s, by)
	if !ok {
		return false
	}
	n.turnTo(by)
	now := w.now()
	n.play(line.Emote, line.EmoteFor, now)
	s.step++
	w.showAll(s)
	if s.mode == protocol.ModePublic && !now.Before(n.heardAt.Add(w.cfg.Talk.PublicEvery)) {
		n.heardAt = now
		w.npcSays(n, line.Say)
	}
	return w.open(s)
}

func (w *World) open(s *session) bool {
	_, ok := w.sessions[s.id]
	return ok
}

// line renders the conversation's line for m; a line that cannot be rendered ends the conversation (and an
// ended conversation renders nothing).
func (w *World) line(s *session, m *member) (dialogue.Line, bool) {
	if !w.open(s) {
		return dialogue.Line{}, false
	}
	l, err := w.cfg.Dialogue.Line(s.npc.spec.Dialogue, s.node, m.speaker())
	if err != nil {
		w.log.Error("conversation broke", "npc", s.npc.spec.Key, "node", s.node, "err", err)
		w.endTalk(s, protocol.EndDone)
		return dialogue.Line{}, false
	}
	return l, true
}

// showAll sends every player of the conversation where it is.
func (w *World) showAll(s *session) { w.showTo(s, slices.Clone(s.members)) }

// show sends one player of the conversation where it is.
func (w *World) show(s *session, m *member) { w.showTo(s, []*member{m}) }

// showTo sends these players of the conversation where it is: each their own variant of the line, with the
// leader's answers (the leader picks them). A line that cannot be rendered ends the conversation instead.
func (w *World) showTo(s *session, to []*member) {
	if len(s.members) == 0 {
		return
	}
	lead := s.members[0]
	l, ok := w.line(s, lead)
	if !ok {
		return
	}
	msg := protocol.Interaction{T: protocol.TypeInteraction, Sid: s.id, N: s.npc.id, Mode: s.mode, Step: s.step, Size: len(s.members),
		Choices: l.Choices}
	var slow []*Peer
	for _, m := range to {
		msg.Lead, msg.Say = m == lead, l.Say
		if m != lead {
			mine, ok := w.line(s, m)
			if !ok {
				break // the conversation ended
			}
			msg.Say = mine.Say
		}
		if !w.offer(m.peer, protocol.Encode(msg)) {
			slow = append(slow, m.peer)
		}
	}
	w.dropSlow(slow)
}

// leaveTalk takes m out of its conversation and tells it why ("" sends nothing: it has left the world). The
// conversation ends with its last player; a group carries on, led by the next one.
func (w *World) leaveTalk(m *member, reason string) {
	s := m.talk
	if s == nil {
		return
	}
	m.talk = nil
	if i := slices.Index(s.members, m); i >= 0 {
		s.members = slices.Delete(s.members, i, i+1)
	}
	if reason != "" {
		w.sendTo(m.peer, protocol.Encode(protocol.InteractionEnd{T: protocol.TypeInteractionEnd, Sid: s.id, Reason: reason}))
	}
	if len(s.members) == 0 {
		w.closeSession(s)
		return
	}
	w.showAll(s)
}

// endTalk ends a conversation for all its players.
func (w *World) endTalk(s *session, reason string) {
	if _, open := w.sessions[s.id]; !open {
		return
	}
	members := s.members
	s.members = nil
	w.closeSession(s)
	b := protocol.Encode(protocol.InteractionEnd{T: protocol.TypeInteractionEnd, Sid: s.id, Reason: reason})
	var slow []*Peer
	for _, m := range members {
		m.talk = nil
		if !w.offer(m.peer, b) {
			slow = append(slow, m.peer)
		}
	}
	w.dropSlow(slow)
}

func (w *World) closeSession(s *session) {
	if _, open := w.sessions[s.id]; !open {
		return
	}
	delete(w.sessions, s.id)
	n := s.npc
	n.talks--
	if n.group == s {
		n.group = nil
	}
	// it keeps looking at the last speaker a moment before turning back
	n.lookUntil = w.now().Add(w.cfg.Talk.LookFor)
}

// walkAways ends the conversation of every player who walked further than the NPC's reach plus Slack. One
// leaving can drop a slow peer that is also on the list; leaveTalk does nothing for a player no longer talking.
func (w *World) walkAways() {
	var far []*member
	for _, s := range w.sessions {
		limit := s.npc.spec.Reach + w.cfg.Talk.Slack
		for _, m := range s.members {
			if s.npc.dist2(m) > limit*limit {
				far = append(far, m)
			}
		}
	}
	for _, m := range far {
		w.leaveTalk(m, protocol.EndFar)
	}
}

func (w *World) signal(m *member, n *npc, signal string) {
	w.log.Debug("dialogue signal", "player", m.id, "npc", n.spec.Key, "signal", signal)
	if w.cfg.OnSignal != nil {
		w.cfg.OnSignal(Signal{Character: m.character, Player: m.id, NPC: n.spec.Key, Signal: signal})
	}
}

// result answers a command that carried a request id; code "" is success.
func (w *World) result(p *Peer, rid uint32, code string) {
	if rid == 0 {
		return
	}
	w.sendTo(p, protocol.Encode(protocol.Result{T: protocol.TypeResult, Rid: rid, OK: code == "", Code: code}))
}

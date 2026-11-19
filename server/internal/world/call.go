package world

import (
	"context"
	"errors"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/protocol"
)

// ContactBook says which numbers a character has (store.DB in production).
type ContactBook interface {
	HasContact(ctx context.Context, character, contact string) (bool, error)
}

// errNoNumber: the character does not have the number it called.
var errNoNumber = errors.New("world: no such number")

// onCall starts a phone call (docs/design/12-phone.md): a conversation with a contact wherever both are, in the
// dialogue the contact speaks, from its call node. It needs a phone and the number; distance does not matter. A
// player is in one conversation at a time, so a call ends the one it is in, as starting another does.
func (w *World) onCall(p *Peer, m *member, msg protocol.ClientMsg) {
	n := w.byKey[msg.Contact]
	switch {
	case w.cfg.Dialogue == nil || w.cfg.Contacts == nil:
		w.result(p, msg.Rid, protocol.ResMute)
		return
	case !m.caps.Has(content.CapPhone):
		w.result(p, msg.Rid, protocol.ResPhone)
		return
	case n == nil || m.character == "":
		// a guest has no numbers
		w.result(p, msg.Rid, protocol.ResTarget)
		return
	}
	node, ok := w.cfg.Dialogue.Call(n.spec.Dialogue)
	if !ok {
		w.result(p, msg.Rid, protocol.ResMute)
		return
	}
	character, book := m.character, w.cfg.Contacts
	w.jobs.do(w, character, func(ctx context.Context) error {
		has, err := book.HasContact(ctx, character, n.spec.Key)
		if err == nil && !has {
			err = errNoNumber
		}
		return err
	}, func(err error) { w.called(p, m, n, node, msg.Rid, err) })
}

// called goes on with a call once the store has said the number is the character's.
func (w *World) called(p *Peer, m *member, n *npc, node string, rid uint32, err error) {
	switch {
	case w.members[p] != m:
		return // it left meanwhile
	case errors.Is(err, errNoNumber):
		w.result(p, rid, protocol.ResTarget)
		return
	case err != nil:
		w.log.Error("call: number lookup failed", "npc", n.spec.Key, "err", err)
		w.result(p, rid, protocol.ResBusy)
		return
	}
	if s := m.talk; s != nil {
		if s.call && s.npc == n {
			// already on this call (a press while the answer was on its way): the line again
			w.show(s, m)
			w.result(p, rid, "")
			return
		}
		w.leaveTalk(m, protocol.EndOther)
		if w.members[p] != m {
			return
		}
	}
	w.nextSession++
	s := &session{id: w.nextSession, npc: n, mode: protocol.ModePrivate, members: []*member{m}, node: node, call: true}
	w.sessions[s.id] = s
	m.talk = s
	if !w.speak(s, m) {
		w.result(p, rid, protocol.ResMute)
		return
	}
	w.result(p, rid, "")
}

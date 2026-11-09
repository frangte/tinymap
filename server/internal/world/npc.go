package world

import (
	"math"
	"math/rand/v2"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/protocol"
)

// NPCSpec is a non-player character to place (server/content/npcs.json). One NPC is one shared figure: everyone
// sees the same one, and any number of players can talk to it at once, each in a conversation of their own (or
// all in one, for a group NPC).
type NPCSpec struct {
	// Key is its content id; the game draws it from its own copy of npcs.json
	Key  string
	Name string
	// X, Z and H are where it stands (three.js metres; Y comes from the floor) and the heading it rests at
	X, Z, H float64
	// Mode is protocol.ModePrivate, ModePublic or ModeGroup
	Mode string
	// Dialogue is the graph it speaks
	Dialogue string
	// Reach is how close a player must stand to start talking (m)
	Reach float64
	// Barks are what it calls out to passers-by now and then
	Barks []protocol.Text
}

// Talk tunes conversations and what NPCs do in them.
type Talk struct {
	// Slack: a conversation ends once its player stands further than the NPC's reach plus this (m)
	Slack float64
	// TurnRate is how fast an NPC turns toward whoever spoke last (rad/s)
	TurnRate float64
	// LookFor is how long an NPC keeps facing the last speaker after a conversation, before it turns back
	LookFor time.Duration
	// VoiceRadius is how far an NPC's calls and public lines carry (m, on the ground)
	VoiceRadius float64
	// BarkEvery is how often an NPC with calls makes one while someone is near enough to hear it, plus up to
	// BarkJitter more
	BarkEvery, BarkJitter time.Duration
	// MaxGroup is how many players a group conversation takes
	MaxGroup int
	// PublicEvery: a public NPC's conversation lines are heard nearby at most this often
	PublicEvery time.Duration
}

func DefaultTalk() Talk {
	return Talk{
		Slack:       2,
		TurnRate:    4,
		LookFor:     5 * time.Second,
		VoiceRadius: 14,
		BarkEvery:   60 * time.Second,
		BarkJitter:  30 * time.Second,
		MaxGroup:    8,
		PublicEvery: 3 * time.Second,
	}
}

// barkRetry is how soon an NPC with nobody around to hear its call looks again.
const barkRetry = 5 * time.Second

type npc struct {
	entity
	spec NPCSpec
	// face is the heading it turns toward: the last speaker's direction, or its rest heading
	face float64
	// lookUntil: once nobody talks to it, it keeps facing the last speaker until then
	lookUntil time.Time
	// emoteUntil is when the emote it plays (in its flags) ends
	emoteUntil time.Time
	// talks counts the conversations with it; group is its open group conversation, nil for none
	talks int
	group *session
	// barkAt is when it next calls out (zero: not scheduled yet), nextBark which call it makes
	barkAt   time.Time
	nextBark int
	// heardAt is when a line of a public conversation with it was last heard nearby
	heardAt time.Time
}

// addNPC places an NPC on the floor where its spec says, or on the nearest free spot.
func (w *World) addNPC(spec NPCSpec) {
	x, z := spec.X, spec.Z
	if fx, fz, ok := w.grid.NearestFit(x, z, bodyRadius, settleSearch); ok {
		if math.Hypot(fx-x, fz-z) > 0.05 {
			w.log.Warn("npc moved to a free spot", "npc", spec.Key, "dx", fx-x, "dz", fz-z)
		}
		x, z = fx, fz
	} else {
		w.log.Warn("npc stands where nobody can walk", "npc", spec.Key)
	}
	y, _ := w.grid.Floor(x, z)
	w.nextID++
	n := &npc{entity: entity{id: w.nextID, pose: protocol.Pose{X: x, Y: y, Z: z, H: spec.H}, poseAt: w.now()}, spec: spec, face: spec.H}
	w.npcs = append(w.npcs, n)
	w.npcByID[n.id] = n
}

func (w *World) npcList() []protocol.NPC {
	out := make([]protocol.NPC, 0, len(w.npcs))
	for _, n := range w.npcs {
		out = append(out, protocol.NPC{ID: n.id, Key: n.spec.Key, Pose: protocol.PoseArray(n.pose)})
	}
	return out
}

// tick runs once a snapshot: conversations whose player walked away end, every NPC turns, finishes its emote and
// calls out, and players' whereabouts are published.
func (w *World) tick() {
	now := w.now()
	dt := now.Sub(w.lastTick).Seconds()
	if w.lastTick.IsZero() || dt > 0.5 {
		dt = w.cfg.SnapInterval.Seconds()
	}
	w.lastTick = now
	w.walkAways()
	for _, n := range w.npcs {
		w.behave(n, now, dt)
	}
	w.observePlaces(now)
	for _, mod := range w.modules {
		mod.Tick(now)
	}
}

func (w *World) behave(n *npc, now time.Time, dt float64) {
	if n.talks == 0 && !now.Before(n.lookUntil) {
		n.face = n.spec.H
	}
	if d := wrapAngle(n.face - n.pose.H); d != 0 {
		step := w.cfg.Talk.TurnRate * dt
		if math.Abs(d) <= step {
			n.pose.H = n.face
		} else {
			n.pose.H = wrapAngle(n.pose.H + math.Copysign(step, d))
		}
		n.changed(now)
	}
	if n.flags&protocol.EmoteMask != 0 && !now.Before(n.emoteUntil) {
		n.flags &^= protocol.EmoteMask
		n.changed(now)
	}
	w.bark(n, now)
}

// changed marks the NPC for a new snapshot row.
func (n *npc) changed(now time.Time) {
	n.view.moved = true
	n.poseAt = now
}

// turnTo makes the NPC face a player (it turns there over the next ticks).
func (n *npc) turnTo(m *member) {
	dx, dz := m.pose.X-n.pose.X, m.pose.Z-n.pose.Z
	if dx*dx+dz*dz > 0.01 {
		n.face = math.Atan2(dx, dz)
	}
}

// play starts an emote (a code from emotes.json) for d; a code of 0 plays nothing.
func (n *npc) play(code int, d time.Duration, now time.Time) {
	if code <= 0 || code > protocol.EmoteMask>>protocol.EmoteShift {
		return
	}
	n.flags = n.flags&^protocol.EmoteMask | code<<protocol.EmoteShift
	n.emoteUntil = now.Add(d)
	n.changed(now)
}

// bark makes an NPC with calls make one now and then, when nobody is talking to it and somebody is near enough
// to hear it.
func (w *World) bark(n *npc, now time.Time) {
	t := w.cfg.Talk
	if len(n.spec.Barks) == 0 || n.talks > 0 {
		return
	}
	if n.barkAt.IsZero() {
		n.barkAt = now.Add(randomUpTo(t.BarkJitter))
		return
	}
	if now.Before(n.barkAt) {
		return
	}
	if !w.anyoneNear(n, t.VoiceRadius) {
		n.barkAt = now.Add(barkRetry)
		return
	}
	w.npcSays(n, n.spec.Barks[n.nextBark%len(n.spec.Barks)])
	n.nextBark++
	n.barkAt = now.Add(t.BarkEvery + randomUpTo(t.BarkJitter))
}

func randomUpTo(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	return rand.N(d)
}

func (w *World) anyoneNear(n *npc, r float64) bool {
	for _, m := range w.members {
		if n.dist2(m) <= r*r {
			return true
		}
	}
	return false
}

func (n *npc) dist2(m *member) float64 {
	dx, dz := m.pose.X-n.pose.X, m.pose.Z-n.pose.Z
	return dx*dx + dz*dz
}

// npcSays sends a line of the NPC as nearby chat to everyone within its voice: the game shows it in the chat
// log and as a bubble over the NPC. Like players' nearby lines, it is not kept in the history.
func (w *World) npcSays(n *npc, t protocol.Text) {
	text := protocol.CleanChat(t.VI)
	if text == "" {
		return
	}
	b := protocol.Encode(protocol.ChatLine{T: protocol.TypeChat, ID: n.id, Name: n.spec.Name, NPC: true, Ch: protocol.ChannelNearby,
		Text: text, EN: protocol.CleanChat(t.EN), At: w.now().UnixMilli()})
	r2 := w.cfg.Talk.VoiceRadius * w.cfg.Talk.VoiceRadius
	var slow []*Peer
	for p, m := range w.members {
		if n.dist2(m) <= r2 && !w.offer(p, b) {
			slow = append(slow, p)
		}
	}
	w.dropSlow(slow)
}

// dropSlow removes peers whose queue was full, after a loop that could not remove them as it went.
func (w *World) dropSlow(slow []*Peer) {
	for _, p := range slow {
		if m, ok := w.members[p]; ok {
			w.log.Warn("dropping slow client", "id", m.id)
			w.remove(p)
		}
	}
}

func wrapAngle(a float64) float64 { return math.Atan2(math.Sin(a), math.Cos(a)) }

// NPCsFrom turns the NPCs of the game data into specs to place.
func NPCsFrom(c *content.Catalog) []NPCSpec {
	out := make([]NPCSpec, 0, len(c.NPCs))
	for _, n := range c.NPCs {
		x, z, h := n.ThreePose()
		out = append(out, NPCSpec{Key: n.ID, Name: n.Name, X: x, Z: z, H: h, Mode: n.Mode, Dialogue: n.Dialogue, Reach: n.Reach,
			Barks: n.Barks})
	}
	return out
}

// EmoteNames maps the emote codes of the game data to their ids, for emote events.
func EmoteNames(c *content.Catalog) map[int]string {
	out := make(map[int]string, len(c.Emotes))
	for _, e := range c.Emotes {
		out[e.Code] = e.ID
	}
	return out
}

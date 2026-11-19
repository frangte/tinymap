package world

import (
	"math"
	"slices"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/protocol"
)

// What the world observes about players, published as events (server/internal/events) for quests and
// achievements. Everything here comes from what the server accepted: a refused move never enters a place, a
// stripped emote is never played.

const (
	// enterAfter is how long a player must stay in a place before it counts as entered
	enterAfter = 2 * time.Second
	// timeEvery: while a player stays in a place, its time there is published in steps of this
	timeEvery = 5 * time.Second
	// placeSlack: a place a player is in reaches this much further before they leave it, so walking along its
	// edge does not enter it over and over (the game's label does the same)
	placeSlack = 3
	// greetReach is how close another player must be for an emote to greet them (m), and greetWindow how soon
	// they must play the same emote back for it to count as a greeting
	greetReach  = 5
	greetWindow = 20 * time.Second
	// rideStep: distance ridden is published in steps of this (m)
	rideStep = 100
)

// placeShape is a place from places.json, ready to measure against.
type placeShape struct {
	id, kind string
	rank     int
	// pts in three.js x, z (Blender (X, Y) is three (x, -y))
	pts [][2]float64
	r   float64
}

// whereabouts is which place a member is in, as far as events go.
type whereabouts struct {
	place     *placeShape
	since     time.Time
	announced bool
	// timeAt is when its time there was last published
	timeAt time.Time
	// rode is the distance ridden not published yet (m)
	rode float64
	// emote is the last emote played (a code) and emoteAt when, for greetings
	emote   int
	emoteAt time.Time
}

var placeRank = map[content.PlaceKind]int{content.Landmark: 0, content.Street: 1, content.Area: 2, content.District: 3}

func shapesOf(places []content.Place) []placeShape {
	out := make([]placeShape, 0, len(places))
	for _, p := range places {
		s := placeShape{id: p.ID, kind: string(p.Kind), rank: placeRank[p.Kind], r: p.R}
		for _, pt := range p.Pts {
			s.pts = append(s.pts, [2]float64{pt[0], -pt[1]})
		}
		out = append(out, s)
	}
	return out
}

// placeAt is the place a point is in, the same way the game labels it (src/game/places.ts placeAt): landmarks
// before streets before areas before districts, then the nearest relative to its reach.
func (w *World) placeAt(x, z float64, current *placeShape) *placeShape {
	var best *placeShape
	bestScore := math.Inf(1)
	for i := range w.places {
		p := &w.places[i]
		reach := p.r
		if p == current {
			reach += placeSlack
		}
		d := p.distance(x, z)
		if d > reach {
			continue
		}
		if score := float64(p.rank)*10 + d/reach; score < bestScore {
			best, bestScore = p, score
		}
	}
	return best
}

func (p *placeShape) distance(x, z float64) float64 {
	if len(p.pts) == 1 {
		return math.Hypot(x-p.pts[0][0], z-p.pts[0][1])
	}
	d := math.Inf(1)
	for i := 1; i < len(p.pts); i++ {
		d = math.Min(d, segDist(x, z, p.pts[i-1], p.pts[i]))
	}
	return d
}

func segDist(x, z float64, a, b [2]float64) float64 {
	vx, vz := b[0]-a[0], b[1]-a[1]
	l2 := vx*vx + vz*vz
	t := 0.0
	if l2 > 0 {
		t = math.Max(0, math.Min(1, ((x-a[0])*vx+(z-a[1])*vz)/l2))
	}
	return math.Hypot(x-(a[0]+vx*t), z-(a[1]+vz*t))
}

// observePlaces runs once a tick: who entered a place, and who stayed another few seconds in one.
func (w *World) observePlaces(now time.Time) {
	if len(w.places) == 0 || w.cfg.OnEvent == nil {
		return
	}
	for _, m := range w.members {
		if m.character == "" {
			continue
		}
		wh := &m.where
		p := w.placeAt(m.pose.X, m.pose.Z, wh.place)
		if p != wh.place {
			*wh = whereabouts{place: p, since: now, rode: wh.rode, emote: wh.emote, emoteAt: wh.emoteAt}
			continue
		}
		if p == nil {
			continue
		}
		riding := m.riding != nil
		if !wh.announced {
			if now.Sub(wh.since) >= enterAfter {
				wh.announced, wh.timeAt = true, now
				w.publish(events.Event{Kind: events.PlaceEntered, Character: m.character, Place: p.id, PlaceKind: p.kind, Riding: riding})
			}
			continue
		}
		if now.Sub(wh.timeAt) >= timeEvery {
			wh.timeAt = wh.timeAt.Add(timeEvery)
			w.publish(events.Event{Kind: events.PlaceTime, Character: m.character, Place: p.id, PlaceKind: p.kind, Riding: riding,
				Seconds: int(timeEvery / time.Second)})
		}
	}
}

// observeMove follows an accepted move: a new emote, and the distance ridden.
func (w *World) observeMove(m *member, from protocol.Pose, oldFlags int) {
	if m.character == "" || w.cfg.OnEvent == nil {
		return
	}
	if code := protocol.EmoteOf(m.flags); code != 0 && code != protocol.EmoteOf(oldFlags) {
		w.emoted(m, code)
	}
	if m.riding != nil {
		m.where.rode += math.Hypot(m.pose.X-from.X, m.pose.Z-from.Z)
		for m.where.rode >= rideStep {
			m.where.rode -= rideStep
			w.publish(events.Event{Kind: events.Rode, Character: m.character, Metres: rideStep, Vehicle: m.riding.kind})
		}
	}
}

// emoted publishes an emote, and a greeting with everyone close by who played the same one at m in the last
// greetWindow: both sides of it are greeted. Waving at a crowd that does not wave back greets nobody.
func (w *World) emoted(m *member, code int) {
	id := w.cfg.Emotes[code]
	if id == "" {
		return
	}
	now := w.now()
	ev := events.Event{Kind: events.Emote, Character: m.character, Emote: id}
	if p := m.where.place; p != nil {
		ev.Place, ev.PlaceKind = p.id, p.kind
	}
	w.publish(ev)
	var partners []string
	r2 := float64(greetReach * greetReach)
	for _, o := range w.members {
		if o == m || o.character == "" || o.character == m.character || o.where.emote != code || now.Sub(o.where.emoteAt) > greetWindow {
			continue
		}
		if dx, dz := o.pose.X-m.pose.X, o.pose.Z-m.pose.Z; dx*dx+dz*dz > r2 {
			continue
		}
		partners = append(partners, o.character)
		w.publish(events.Event{Kind: events.Greeted, Character: o.character, Emote: id, Partners: []string{m.character}})
	}
	m.where.emote, m.where.emoteAt = code, now
	if len(partners) > 0 {
		slices.Sort(partners)
		w.publish(events.Event{Kind: events.Greeted, Character: m.character, Emote: id, Partners: slices.Compact(partners)})
	}
}

// publish hands an event to whoever listens (Config.OnEvent), which must not block the world.
func (w *World) publish(ev events.Event) {
	if w.cfg.OnEvent != nil && ev.Character != "" {
		w.cfg.OnEvent(ev)
	}
}

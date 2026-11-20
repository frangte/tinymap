// Package playtest makes a test character in a chosen story state, for the Xưởng truyện's "Chơi thử từ đây"
// (docs/design/14-story-tools.md): a fresh account of its own, with the quests the author names done the way
// playing them would have left them, so the author can open the game at any point of the story. It exists only
// behind the dev route (SG_DEV_TOOLS), never touches an existing account, and keeps no rules of its own: quests
// end through quest.Engine.Complete, money and items through the store's own rows.
package playtest

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/identity"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/quest"
	"saigon-noir/server/internal/ridehail"
)

// NamePrefix marks a test character's name, so it is never mistaken for a player's.
const NamePrefix = "Thử: "

const (
	// nameMax is the longest name the game takes (src/game/profile.ts NAME_MAX), prefix included
	nameMax = 16
	// doneMax bounds the quests one request may count as done; xuMax the money it may add
	doneMax = 300
	xuMax   = 100_000_000
	// standDistance is how far from an NPC the test character stands, facing it (m)
	standDistance = 1.5
	// defaultBody is the body the game starts a new player in (src/game/outfits.ts DEFAULT_LOOK)
	defaultBody = "man"
)

// ErrInvalid wraps every refusal of a request, so the route can answer 400 with the reason.
var ErrInvalid = errors.New("playtest: invalid request")

// DoneQuest is a quest to count as done, with the outcome it ended with (for a quest that has outcomes).
type DoneQuest struct {
	Quest   string `json:"quest"`
	Outcome string `json:"outcome,omitempty"`
}

// Request is the state to put a test character in.
type Request struct {
	// Archetype is the origin (archetypes.json)
	Archetype string `json:"archetype"`
	// Done are the quests to count as done; each one's requires must be in the list too
	Done []DoneQuest `json:"done"`
	// Flags are extra story flags to hold, besides the ones the done quests set
	Flags []string `json:"flags"`
	// Items are item ids (items.json) to own, free
	Items []string `json:"items"`
	// Xu is money to add to the starting wallet
	Xu int64 `json:"xu"`
	// Stand is a place id or an NPC id: where the character starts ("" for its archetype's start)
	Stand string `json:"stand"`
	// Label follows the name prefix ("" for a default)
	Label string `json:"label"`
}

// Result is the test character and the token that logs in as it.
type Result struct {
	Token     string         `json:"token"`
	Character string         `json:"character"`
	Name      string         `json:"name"`
	Archetype string         `json:"archetype"`
	Look      protocol.Look  `json:"look"`
	Pose      *protocol.Pose `json:"pose,omitempty"`
	Balance   int64          `json:"balance"`
	// Active are the quests running once the done ones are applied
	Active []string `json:"active"`
	Flags  []string `json:"flags"`
}

// Accounts is the identity service: guests, their characters and where they stand.
type Accounts interface {
	NewGuest(ctx context.Context) (string, error)
	Arrive(ctx context.Context, token string, p identity.Profile) (identity.Arrived, error)
	SavePose(ctx context.Context, characterID string, p protocol.Pose) error
}

// Store is what the maker writes: quest state, the wallet, items and money (store.DB).
type Store interface {
	quest.Store
	ridehail.Wallets
	GiveItem(ctx context.Context, character, item string, grants []string, at int64) error
	GiveXu(ctx context.Context, character string, xu int64, reason string, at int64) error
}

// Maker makes test characters.
type Maker struct {
	Catalog  *content.Catalog
	Engine   *quest.Engine
	Accounts Accounts
	Store    Store
	Now      func() time.Time
}

// plan is a request that checked out against the content.
type plan struct {
	req   Request
	arch  content.Archetype
	order []DoneQuest
	items []content.Item
	pose  *protocol.Pose
}

// Make makes a new account and its character and puts it in the requested state. A refused request makes
// nothing; a request that fails halfway leaves a test account behind, which no player will ever log into.
func (m *Maker) Make(ctx context.Context, req Request) (Result, error) {
	p, err := m.check(req)
	if err != nil {
		return Result{}, err
	}
	now := m.Now().UnixMilli()
	look := protocol.Look{Body: defaultBody, Outfit: p.arch.Look.Outfit, Hat: p.arch.Look.Hat}
	name := testName(req.Label)
	token, err := m.Accounts.NewGuest(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("playtest: %w", err)
	}
	found, err := m.Accounts.Arrive(ctx, token, identity.Profile{Name: name, Look: look, Archetype: p.arch.ID})
	if err != nil {
		return Result{}, fmt.Errorf("playtest: %w", err)
	}
	character := found.ID
	purse := ridehail.Purse{Wallets: m.Store, Cfg: m.Catalog.Ridehail, Now: m.Now}
	if _, err := purse.Arrive(ctx, character); err != nil {
		return Result{}, fmt.Errorf("playtest: wallet: %w", err)
	}
	player := quest.Player{Tags: m.Catalog.TagsOf(p.arch.ID), Caps: m.Catalog.Grants(p.arch.ID)}
	if err := m.give(ctx, character, p, player.Caps, now); err != nil {
		return Result{}, err
	}
	book, err := m.apply(ctx, character, p, player, now)
	if err != nil {
		return Result{}, err
	}
	if p.pose != nil {
		if err := m.Accounts.SavePose(ctx, character, *p.pose); err != nil {
			return Result{}, fmt.Errorf("playtest: %w", err)
		}
	}
	balance, err := purse.Arrive(ctx, character)
	if err != nil {
		return Result{}, fmt.Errorf("playtest: wallet: %w", err)
	}
	return Result{Token: token, Character: character, Name: name, Archetype: p.arch.ID, Look: look, Pose: p.pose, Balance: balance,
		Active: sortedKeys(book.Active()), Flags: sortedKeys(book.Flags)}, nil
}

// give puts the items and the extra money in place; the capabilities the items grant join caps.
func (m *Maker) give(ctx context.Context, character string, p plan, caps capability.Set, now int64) error {
	for _, it := range p.items {
		if err := m.Store.GiveItem(ctx, character, it.ID, it.Grants, now); err != nil {
			return fmt.Errorf("playtest: %w", err)
		}
		for _, k := range it.Grants {
			caps[k] = struct{}{}
		}
	}
	if p.req.Xu > 0 {
		if err := m.Store.GiveXu(ctx, character, p.req.Xu, "playtest", now); err != nil {
			return fmt.Errorf("playtest: %w", err)
		}
	}
	return nil
}

// apply stores the extra flags, then completes the quests one transaction each, as playing them would, and last
// starts the quests that are open now. A capability a reward grants is held for the quests after it.
func (m *Maker) apply(ctx context.Context, character string, p plan, player quest.Player, now int64) (*quest.Book, error) {
	book := quest.NewBook(quest.Saved{})
	store := func(out quest.Outcome) error {
		if out.Empty() {
			return nil
		}
		if _, err := m.Store.SaveQuests(ctx, character, out.Commit(now)); err != nil {
			return fmt.Errorf("playtest: %w", err)
		}
		for _, g := range out.Grants {
			if g.Capability != "" {
				player.Caps[g.Capability] = struct{}{}
			}
		}
		return nil
	}
	if len(p.req.Flags) > 0 {
		var flags quest.Outcome
		flags.Flags = p.req.Flags
		for _, f := range p.req.Flags {
			book.Flags[f] = true
		}
		if err := store(flags); err != nil {
			return nil, err
		}
	}
	for _, d := range p.order {
		out, err := m.Engine.Complete(player, book, character, d.Quest, d.Outcome, now)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
		}
		if err := store(out); err != nil {
			return nil, err
		}
	}
	return book, store(m.Engine.Refresh(player, book, now))
}

// check validates the request against the content and puts the done quests in an order they can be completed in.
func (m *Maker) check(req Request) (plan, error) {
	c := m.Catalog
	p := plan{req: req}
	var ok bool
	if p.arch, ok = c.Archetype(req.Archetype); !ok {
		return p, invalid("unknown archetype %q", req.Archetype)
	}
	if req.Xu < 0 || req.Xu > xuMax {
		return p, invalid("xu must be between 0 and %d", xuMax)
	}
	if len(req.Done) > doneMax {
		return p, invalid("at most %d quests done", doneMax)
	}
	byQuest := map[string]DoneQuest{}
	for _, d := range req.Done {
		if err := m.checkDone(d, byQuest); err != nil {
			return p, err
		}
		byQuest[d.Quest] = d
	}
	var err error
	if p.order, err = m.sorted(req.Done, byQuest); err != nil {
		return p, err
	}
	setters := c.FlagSetters()
	for _, f := range req.Flags {
		if _, known := setters[f]; !known {
			return p, invalid("flag %q is not set by any quest", f)
		}
	}
	for _, id := range req.Items {
		it, known := c.Item(id)
		if !known {
			return p, invalid("unknown item %q", id)
		}
		p.items = append(p.items, it)
	}
	if p.pose, err = m.stand(req.Stand); err != nil {
		return p, err
	}
	return p, nil
}

func (m *Maker) checkDone(d DoneQuest, seen map[string]DoneQuest) error {
	q, ok := m.quest(d.Quest)
	switch {
	case !ok:
		return invalid("unknown quest %q", d.Quest)
	case q.Repeat != content.RepeatOnce:
		return invalid("quest %q repeats: only a quest done once can count as done", d.Quest)
	}
	if _, dup := seen[d.Quest]; dup {
		return invalid("quest %q is listed twice", d.Quest)
	}
	if len(q.Outcomes) == 0 {
		if d.Outcome != "" {
			return invalid("quest %q has no outcomes, not %q", d.Quest, d.Outcome)
		}
		return nil
	}
	if !slices.ContainsFunc(q.Outcomes, func(o content.QuestOutcome) bool { return o.ID == d.Outcome }) {
		return invalid("quest %q has no outcome %q (it needs one of: %s)", d.Quest, d.Outcome, outcomeIDs(q))
	}
	return nil
}

// sorted orders the done quests so each comes after the ones it requires; a quest that requires one that is not
// in the list is refused, since the character could never have done it.
func (m *Maker) sorted(done []DoneQuest, byQuest map[string]DoneQuest) ([]DoneQuest, error) {
	for _, d := range done {
		q, _ := m.quest(d.Quest)
		for _, r := range q.Requires {
			if _, ok := byQuest[r]; !ok {
				return nil, invalid("quest %q needs %q done first", d.Quest, r)
			}
		}
	}
	var out []DoneQuest
	placed := map[string]bool{}
	for len(out) < len(done) {
		before := len(out)
		for _, d := range done {
			q, _ := m.quest(d.Quest)
			if placed[d.Quest] || slices.ContainsFunc(q.Requires, func(r string) bool { return !placed[r] }) {
				continue
			}
			placed[d.Quest] = true
			out = append(out, d)
		}
		if len(out) == before {
			return nil, invalid("the done quests need each other: no order to do them in")
		}
	}
	return out, nil
}

// stand is the pose of a place or of the spot beside an NPC, facing it; nil for "" (the archetype's start).
func (m *Maker) stand(id string) (*protocol.Pose, error) {
	if id == "" {
		return nil, nil
	}
	if n, ok := m.Catalog.NPCByID(id); ok && len(n.At) == 2 && len(n.Face) == 2 {
		dx, dy := n.Face[0]-n.At[0], n.Face[1]-n.At[1]
		d := math.Hypot(dx, dy)
		if d == 0 {
			dx, dy, d = 0, 1, 1
		}
		reach := math.Min(standDistance, math.Max(n.Reach/2, 0.5))
		at := []float64{n.At[0] + dx/d*reach, n.At[1] + dy/d*reach}
		x, z, h := content.Spawn{At: at, Face: n.At}.ThreePose()
		return &protocol.Pose{X: x, Z: z, H: h}, nil
	}
	if pl, ok := m.Catalog.Place(id); ok && len(pl.Pts) > 0 && len(pl.Pts[0]) == 2 {
		x, z, _ := content.Spawn{At: pl.Pts[0], Face: pl.Pts[0]}.ThreePose()
		return &protocol.Pose{X: x, Z: z}, nil
	}
	return nil, invalid("%q is neither a place nor an NPC", id)
}

// quest looks the id up in the catalog's list (the engine does the same, so a test can add quests to it).
func (m *Maker) quest(id string) (content.Quest, bool) {
	i := slices.IndexFunc(m.Catalog.Quests, func(q content.Quest) bool { return q.ID == id })
	if i < 0 {
		return content.Quest{}, false
	}
	return m.Catalog.Quests[i], true
}

func testName(label string) string {
	label = strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < ' ' || r == 0x7f {
			return -1
		}
		return r
	}, label))
	if label == "" {
		label = "thử"
	}
	room := nameMax - utf8.RuneCountInString(NamePrefix)
	if r := []rune(label); len(r) > room {
		label = strings.TrimSpace(string(r[:room]))
	}
	return NamePrefix + label
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

func outcomeIDs(q content.Quest) string {
	ids := make([]string, 0, len(q.Outcomes))
	for _, o := range q.Outcomes {
		ids = append(ids, o.ID)
	}
	return strings.Join(ids, ", ")
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

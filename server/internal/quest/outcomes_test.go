package quest

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"slices"
	"testing"
	"time"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/capability"
	"saigon-noir/server/internal/dialogue"
	"saigon-noir/server/internal/events"
	"saigon-noir/server/internal/protocol"
)

// forkCatalog is the game data plus testdata/branching_story.json: a quest with three outcomes, quests that read the
// flags they set, and the dialogue of an NPC that reads them too.
func forkCatalog(t *testing.T) *content.Catalog {
	t.Helper()
	c := catalog(t)
	raw, err := os.ReadFile("testdata/branching_story.json")
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		About     string                      `json:"about"`
		Stamps    []content.Stamp             `json:"stamps"`
		Quests    []content.Quest             `json:"quests"`
		NPC       content.NPC                 `json:"npc"`
		Dialogues map[string]content.Dialogue `json:"dialogues"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&fx); err != nil {
		t.Fatalf("testdata/branching_story.json: %v", err)
	}
	c.Stamps = append(c.Stamps, fx.Stamps...)
	c.Quests = append(c.Quests, fx.Quests...)
	c.NPCs = append(slices.Clone(c.NPCs), fx.NPC)
	for id, d := range fx.Dialogues {
		c.Dialogues[id] = d
	}
	return c
}

func forkRun(t *testing.T) *run {
	t.Helper()
	e, err := NewEngine(forkCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	r := &run{t: t, e: e, who: Player{Caps: capability.Of(content.CapPhone, content.CapSmartphone)}, book: NewBook(Saved{}), now: 1}
	r.collect(r.e.Refresh(r.who, r.book, r.now))
	return r
}

func (r *run) outcomeOf(id string) string {
	if rec := r.book.Records[id]; rec != nil {
		return rec.Outcome
	}
	return ""
}

func (r *run) flags() []string {
	var out []string
	for f := range r.book.Flags {
		out = append(out, f)
	}
	slices.Sort(out)
	return out
}

func liveOutcomes(r *run, id string) []string {
	var q protocol.QuestState
	var p protocol.Progress
	_ = json.Unmarshal(r.e.progress(r.book, nil, r.now), &p)
	for _, s := range p.Quests {
		if s.ID == id {
			q = s
		}
	}
	var out []string
	for _, o := range q.Outcomes {
		out = append(out, o.ID)
	}
	return out
}

func TestOutcomesGoLiveWhenTheObjectivesAreDone(t *testing.T) {
	r := forkRun(t)
	if got := liveOutcomes(r, "test.fork"); got != nil {
		t.Fatalf("outcomes live before the objective: %v", got)
	}
	// an answer picked early counts for nothing: the quest has not asked for it yet
	r.do(signal("test_npc", "hand_over"))
	if r.state("test.fork") != StateActive || r.outcomeOf("test.fork") != "" {
		t.Fatalf("an outcome counted before its turn: %+v", r.book.Records["test.fork"])
	}
	r.do(entered("cho_ben_thanh", "market"))
	if r.state("test.fork") != StateActive {
		t.Fatal("finishing the objective must not end the quest: an outcome has to be picked")
	}
	if got := liveOutcomes(r, "test.fork"); !slices.Equal(got, []string{"keep", "hand_over", "ask"}) {
		t.Fatalf("live outcomes %v", got)
	}
	if len(r.book.Flags) != 0 {
		t.Fatalf("flags before an outcome: %v", r.flags())
	}
}

func TestTheFirstOutcomePickedEndsTheQuestWithItsFlagsAndRewards(t *testing.T) {
	r := forkRun(t)
	r.do(entered("cho_ben_thanh", "market"))
	out := r.do(signal("test_npc", "hand_over"))
	if rec := r.book.Records["test.fork"]; rec.State != StateDone || rec.Outcome != "hand_over" {
		t.Fatalf("record %+v", rec)
	}
	want := []string{"fork_done", "gave_it", "good_name", "test.fork:hand_over"}
	if got := r.flags(); !slices.Equal(got, want) {
		t.Fatalf("flags %v, want %v", got, want)
	}
	slices.Sort(out.Flags)
	if !slices.Equal(out.Flags, want) {
		t.Fatalf("the step reports flags %v", out.Flags)
	}
	// the quest's rewards and the outcome's are separate grants, each given once by its source
	sources := map[string]Grant{}
	for _, g := range out.Grants {
		sources[g.Source] = g
	}
	if g := sources["quest:test.fork:"]; g.Xu != 1000 {
		t.Fatalf("quest grant %+v", g)
	}
	if g := sources["quest:test.fork:outcome:hand_over"]; g.Xu != 5000 || !slices.Equal(g.Stamps, []string{"test_stamp"}) {
		t.Fatalf("outcome grant %+v", g)
	}
	if !r.book.Stamps["test_stamp"] {
		t.Fatal("the outcome's stamp is not in the book")
	}
	// the quest's message and the outcome's
	if len(out.Texts) != 2 || out.Texts[0].Key != "test.fork//done/0" || out.Texts[1].Key != "test.fork//done:hand_over/0" {
		t.Fatalf("texts %+v", out.Texts)
	}
	// the other answers no longer count, and an ended quest keeps its outcome
	r.do(signal("test_npc", "keep"))
	r.do(signal("test_npc", "ask"))
	if r.outcomeOf("test.fork") != "hand_over" || r.book.Flags["kept_it"] {
		t.Fatalf("a later answer changed the ending: %+v, flags %v", r.book.Records["test.fork"], r.flags())
	}
}

func TestFlagsOpenAndCloseQuests(t *testing.T) {
	cases := []struct {
		signal      string
		open        []string
		closedAfter string
	}{
		{"hand_over", []string{"test.after_hand_over", "test.unless_ask"}, "test.after_keep"},
		{"keep", []string{"test.after_keep", "test.unless_ask"}, "test.after_hand_over"},
		{"ask", nil, "test.unless_ask"},
	}
	for _, c := range cases {
		t.Run(c.signal, func(t *testing.T) {
			r := forkRun(t)
			for _, id := range []string{"test.after_hand_over", "test.after_keep", "test.unless_ask"} {
				if _, started := r.book.Records[id]; started {
					t.Fatalf("%s is open before the quest it reads", id)
				}
			}
			r.do(entered("cho_ben_thanh", "market"))
			r.do(signal("test_npc", c.signal))
			for _, id := range c.open {
				if r.state(id) != StateActive {
					t.Errorf("%s should be open after %s", id, c.signal)
				}
			}
			if _, started := r.book.Records[c.closedAfter]; started {
				t.Errorf("%s should stay closed after %s", c.closedAfter, c.signal)
			}
		})
	}
}

func TestAQuestRunningKeepsRunningWhenAFlagCloses(t *testing.T) {
	r := forkRun(t)
	r.do(entered("cho_ben_thanh", "market"))
	r.do(signal("test_npc", "keep"))
	if r.state("test.unless_ask") != StateActive {
		t.Fatal("test.unless_ask should be running")
	}
	// a flag set later cannot take it back
	r.book.Flags["test.fork:ask"] = true
	r.e.Refresh(r.who, r.book, r.now+1)
	if r.state("test.unless_ask") != StateActive {
		t.Fatal("a quest that started keeps running")
	}
}

func TestAQuestWithoutObjectivesHasItsOutcomesLiveAtOnce(t *testing.T) {
	r := forkRun(t)
	if got := liveOutcomes(r, "test.pure_choice"); !slices.Equal(got, []string{"left", "right"}) {
		t.Fatalf("live outcomes %v", got)
	}
	r.do(entered("ho_con_rua", "landmark"))
	if r.outcomeOf("test.pure_choice") != "left" || !r.book.Flags["test.pure_choice:left"] || r.book.Flags["test.pure_choice:right"] {
		t.Fatalf("record %+v, flags %v", r.book.Records["test.pure_choice"], r.flags())
	}
	// the other outcome is closed now
	r.do(entered("bitexco", "landmark"))
	if r.outcomeOf("test.pure_choice") != "left" || r.book.Flags["test.pure_choice:right"] {
		t.Fatal("an outcome counted after the quest ended")
	}
}

func TestWhenTwoOutcomesCompleteTogetherTheFirstInFileOrderWins(t *testing.T) {
	c := forkCatalog(t)
	for i := range c.Quests {
		if c.Quests[i].ID == "test.fork" {
			// the same answer completes "hand_over" and "ask"
			c.Quests[i].Outcomes[2].Params = c.Quests[i].Outcomes[1].Params
		}
	}
	e, err := NewEngine(c)
	if err != nil {
		t.Fatal(err)
	}
	r := &run{t: t, e: e, who: Player{}, book: NewBook(Saved{}), now: 1}
	r.collect(e.Refresh(r.who, r.book, r.now))
	r.do(entered("cho_ben_thanh", "market"))
	r.do(signal("test_npc", "hand_over"))
	if r.outcomeOf("test.fork") != "hand_over" {
		t.Fatalf("ended with %q", r.outcomeOf("test.fork"))
	}
}

func TestAnOutcomeIsRememberedInTheProgressTheGameSees(t *testing.T) {
	r := forkRun(t)
	r.do(entered("cho_ben_thanh", "market"))
	r.do(signal("test_npc", "ask"))
	var p protocol.Progress
	if err := json.Unmarshal(r.e.progress(r.book, nil, r.now), &p); err != nil {
		t.Fatal(err)
	}
	for _, q := range p.Quests {
		if q.ID == "test.fork" && (!q.Done || q.Outcome != "ask" || len(q.Outcomes) != 0) {
			t.Fatalf("done quest as sent: %+v", q)
		}
	}
}

// The dialogue provider only reads flags: what an answer or a line shows follows the flags the quests set.
func TestDialogueFollowsTheFlags(t *testing.T) {
	r := forkRun(t)
	script := dialogue.NewScript(forkCatalog(t))
	speak := func() (protocol.Text, []string) {
		sp := dialogue.Speaker{Name: "Lan", Flags: r.book.Flags}
		l, err := script.Line("fork_talk", "a", sp)
		if err != nil {
			t.Fatal(err)
		}
		var answers []string
		for _, c := range l.Choices {
			answers = append(answers, c.EN)
		}
		return l.Say, answers
	}
	say, answers := speak()
	if say.EN != "What do you have there?" || !slices.Equal(answers, []string{"Keep", "Hand over", "Ask", "Bye"}) {
		t.Fatalf("before: %q %v", say.EN, answers)
	}
	r.do(entered("cho_ben_thanh", "market"))
	r.do(signal("test_npc", "hand_over"))
	say, answers = speak()
	if say.EN != "Good kid." || !slices.Equal(answers, []string{"I handed it over", "Bye"}) {
		t.Fatalf("after handing it over: %q %v", say.EN, answers)
	}
	// the flag lines go by alphabetical order of their keys: "flag:kept_it" before "flag:test.fork:hand_over"
	r.book.Flags["kept_it"] = true
	if say, _ = speak(); say.EN != "You still keep it?" {
		t.Fatalf("with both flags: %q", say.EN)
	}
	// an answer gated by a flag cannot be picked without it (a tampered client)
	sp := dialogue.Speaker{Name: "Lan", Flags: map[string]bool{"fork_done": true}}
	if _, err := script.Choose("fork_talk", "a", sp, 0); err != nil {
		t.Fatalf("the only answer for a speaker without the hand_over flag is 'Bye': %v", err)
	}
	if _, err := script.Choose("fork_talk", "a", sp, 1); err == nil {
		t.Fatal("picked an answer it was not offered")
	}
}

// Signals of one conversation apply in the order they were sent: an answer that completes the last objective and the
// answer that picks an outcome right after it must both count.
func TestSignalsOfOneConversationApplyInOrderAndSetFlagsLive(t *testing.T) {
	store := newMemStore()
	e, err := NewEngine(forkCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	s, w := startServiceWith(t, store, e)
	s.Join("c1", 1, Player{Caps: capability.Of(content.CapPhone, content.CapSmartphone)})
	w.last(t, "c1")
	// all sent back to back, before the worker has taken the first
	s.Submit(events.Event{Kind: events.PlaceEntered, Character: "c1", Place: "cho_ben_thanh", PlaceKind: "market"})
	s.Submit(events.Event{Kind: events.Signal, Character: "c1", NPC: "test_npc", Signal: "hand_over"})
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatalf("no outcome; the world was told flags %v", w.flags["c1"])
		case <-w.received:
		}
		w.mu.Lock()
		held := w.flags["c1"]["test.fork:hand_over"]
		w.mu.Unlock()
		if held {
			break
		}
	}
	p := w.lastFrame("c1")
	if q, _ := questState(p, "test.fork"); !q.Done || q.Outcome != "hand_over" {
		t.Fatalf("progress %+v", q)
	}
	if q, ok := questState(p, "test.after_hand_over"); !ok || q.Done {
		t.Fatal("the quest that reads the flag did not open")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if got := store.saved["c1"].Flags; !slices.Contains(got, "gave_it") || !slices.Contains(got, "test.fork:hand_over") {
		t.Fatalf("stored flags %v", got)
	}
	// and a visit later loads them again, and tells the world
	if !store.granted["c1quest:test.fork:outcome:hand_over"] {
		t.Fatal("the outcome's reward was not given")
	}
}

func TestFlagsAreLoadedOnJoin(t *testing.T) {
	store := newMemStore()
	store.saved["c1"] = &Saved{Flags: []string{"kept_it", "fork_done", "test.fork:keep"},
		Records: []Record{{ID: "test.fork", Version: 1, State: StateDone, Outcome: "keep"}}}
	e, err := NewEngine(forkCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	s, w := startServiceWith(t, store, e)
	s.Join("c1", 1, Player{})
	p := w.last(t, "c1")
	w.mu.Lock()
	flags := w.flags["c1"]
	w.mu.Unlock()
	if !flags["kept_it"] || !flags["test.fork:keep"] {
		t.Fatalf("the world was told flags %v", flags)
	}
	if q, ok := questState(p, "test.after_keep"); !ok {
		t.Fatalf("a quest that reads a held flag is offered on join: %+v", p.Quests)
	} else if q.Done {
		t.Fatal("started done")
	}
}

func (w *fakeWorld) lastFrame(character string) protocol.Progress {
	w.mu.Lock()
	defer w.mu.Unlock()
	f := w.frames[character]
	return f[len(f)-1]
}

func startServiceWith(t *testing.T, store Store, e *Engine) (*Service, *fakeWorld) {
	t.Helper()
	w := newFakeWorld()
	s := NewService(e, store, w, Config{Workers: 2, Queue: 64, StoreWait: time.Second}, discardLog())
	ctx := t.Context()
	go s.Run(ctx)
	return s, w
}

func discardLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

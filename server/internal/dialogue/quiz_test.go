package dialogue

import (
	"slices"
	"testing"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/capability"
)

// ---- Thầy Mười's theory test: ten questions, three wrong answers fail it (docs/design/13-driver.md)

// quiz plays the theory test from its first question, answering each with the right choice (the one that adds
// nothing to the tally) unless wrong says to get it wrong. It returns where the conversation ended up and every
// signal on the way.
func quiz(t *testing.T, s *Script, who Speaker, wrong map[int]bool) (node string, signals []string) {
	t.Helper()
	d, _ := s.catalog.Dialogue("thay_muoi")
	step, err := s.Choose("thay_muoi", "quiz_intro", who, 0)
	if err != nil || step.Next != "q1" {
		t.Fatalf("starting the quiz: %+v, %v", step, err)
	}
	node, signals = step.Next, append(signals, step.Signals...)
	for q := 1; ; q++ {
		n := d.Nodes[node]
		if len(n.Choices) == 0 {
			return node, signals
		}
		pick := -1
		for i, ch := range n.Choices {
			if (ch.Mark != "") == wrong[q] {
				pick = i
				break
			}
		}
		if pick < 0 {
			t.Fatalf("%s: no %v answer to pick", node, wrong[q])
		}
		step, err = s.Choose("thay_muoi", node, who, pick)
		if err != nil {
			t.Fatal(err)
		}
		signals = append(signals, step.Signals...)
		if step.Next == "pass" || step.Next == "fail" {
			return step.Next, signals
		}
		// the explanation, then the next question
		node = step.Next
		step, err = s.Choose("thay_muoi", node, who, 0)
		if err != nil {
			t.Fatal(err)
		}
		signals = append(signals, step.Signals...)
		node = step.Next
		if node == "pass" {
			return node, signals
		}
	}
}

func examiner(t *testing.T) (*Script, Speaker) {
	t.Helper()
	c, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	return NewScript(c), Speaker{Name: "Lan", Caps: capability.Of(content.CapExamTicket), Marks: map[string]int{}}
}

func TestTenRightAnswersPassTheTheory(t *testing.T) {
	s, who := examiner(t)
	node, signals := quiz(t, s, who, nil)
	if node != "pass" || !slices.Equal(signals, []string{content.SignalTheoryStart, content.SignalTheoryPass}) {
		t.Fatalf("ended at %q with %v", node, signals)
	}
}

func TestTwoWrongAnswersStillPass(t *testing.T) {
	s, who := examiner(t)
	node, signals := quiz(t, s, who, map[int]bool{2: true, 9: true})
	if node != "pass" || !slices.Contains(signals, content.SignalTheoryPass) {
		t.Fatalf("ended at %q with %v: eight right is a pass", node, signals)
	}
}

func TestTheThirdWrongAnswerFailsAtOnce(t *testing.T) {
	s, who := examiner(t)
	node, signals := quiz(t, s, who, map[int]bool{1: true, 4: true, 5: true})
	if node != "fail" || slices.Contains(signals, content.SignalTheoryPass) {
		t.Fatalf("ended at %q with %v", node, signals)
	}
	// the attempt started: the ticket is spent whether or not it was passed
	if !slices.Contains(signals, content.SignalTheoryStart) {
		t.Fatalf("signals %v: the start must be sent", signals)
	}
}

func TestARetakeStartsWithAFreshTally(t *testing.T) {
	s, who := examiner(t)
	if node, _ := quiz(t, s, who, map[int]bool{1: true, 2: true, 3: true}); node != "fail" {
		t.Fatalf("first attempt ended at %q", node)
	}
	// the same conversation, started again: the wrong answers of the first attempt do not count
	node, _ := quiz(t, s, who, map[int]bool{1: true, 2: true})
	if node != "pass" {
		t.Fatalf("second attempt ended at %q: the tally was not reset", node)
	}
}

// The way into the quiz and the practical is gated by capabilities, not by the client's good will.
func TestTheExaminersAnswersFollowTheCapabilities(t *testing.T) {
	c, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	s := NewScript(c)
	offered := func(node string, caps ...string) []string {
		l, err := s.Line("thay_muoi", node, Speaker{Caps: capability.Of(caps...)})
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, ch := range l.Choices {
			out = append(out, ch.EN)
		}
		return out
	}
	has := func(list []string, want string) bool { return slices.Contains(list, want) }
	if l := offered("quiz_intro"); has(l, "Ready.") {
		t.Fatalf("the quiz is offered without a ticket: %v", l)
	}
	if l := offered("quiz_intro", content.CapExamTicket); !has(l, "Ready.") {
		t.Fatalf("the quiz is not offered with a ticket: %v", l)
	}
	if l := offered("practical_intro", content.CapExamTicket); has(l, "I am ready.") {
		t.Fatalf("the practical is offered without the theory pass: %v", l)
	}
	if l := offered("practical_intro", content.CapExamTicket, content.CapExamTheory); !has(l, "I am ready.") {
		t.Fatalf("the practical is not offered after the theory pass: %v", l)
	}
	// the right fee for the right person
	if l := offered("intro"); !has(l, "I will pay the test fee.") || has(l, "I will pay the retake fee.") {
		t.Fatalf("a first-timer is offered %v", l)
	}
	if l := offered("intro", content.CapExamRetake); has(l, "I will pay the test fee.") || !has(l, "I will pay the retake fee.") {
		t.Fatalf("someone who failed is offered %v", l)
	}
	if l := offered("greet", content.CapRide); has(l, "I want to take the A1 motorbike licence.") || !has(l, "I already have my licence.") {
		t.Fatalf("a licensed rider is offered %v", l)
	}
}

func TestCapabilityVariantsOfALine(t *testing.T) {
	c, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	s := NewScript(c)
	line := func(arch string, caps ...string) string {
		l, err := s.Line("chu_sau", "greet", Speaker{Name: "Lan", Arch: arch, Caps: capability.Of(caps...)})
		if err != nil {
			t.Fatal(err)
		}
		return l.Say.EN
	}
	driver := line("", "ridehail.drive")
	if driver != "Oh, Crab Bike? How many rides today?" {
		t.Fatalf("a Crab driver is greeted %q", driver)
	}
	if line("", "uniform.crab") == driver {
		t.Fatal("a uniform alone is no variant")
	}
	// the archetype's own line wins over a capability's
	if line("newcomer", "ridehail.drive") == driver {
		t.Fatal("an archetype's variant must come first")
	}
}

package objective

import (
	"encoding/json"
	"errors"
	"fmt"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/events"
)

// pay_fee: pay a fee ({"fee": id}, items.json), each time it is paid counting once. Quests that wait for a fee
// must be offered before the fee can be paid, or a player who paid early would wait for ever.
type payFee struct{}

func init() { Register(payFee{}) }

func (payFee) Kind() string { return "pay_fee" }

func (payFee) Compile(params json.RawMessage, count int, refs Refs) (Matcher, error) {
	var p struct {
		Fee string `json:"fee"`
	}
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	if _, ok := refs.Fee(p.Fee); !ok {
		return nil, fmt.Errorf("unknown fee %q", p.Fee)
	}
	if count > 1 {
		return nil, errOnce
	}
	return feeMatcher{fee: p.Fee}, nil
}

type feeMatcher struct{ fee string }

func (feeMatcher) Listens() []events.Kind { return []events.Kind{events.FeePaid} }

func (m feeMatcher) Advance(ev events.Event, p *Progress, max int) bool {
	return ev.Fee == m.fee && p.Count(1, max)
}

// pass_test: pass a test ({"test": "a1"}, the licence test: server/internal/driving).
type passTest struct{}

func init() { Register(passTest{}) }

func (passTest) Kind() string { return "pass_test" }

func (passTest) Compile(params json.RawMessage, count int, refs Refs) (Matcher, error) {
	var p struct {
		Test string `json:"test"`
	}
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	if p.Test != content.TestA1 {
		return nil, errors.New("test must be \"a1\"")
	}
	if count > 1 {
		return nil, errOnce
	}
	return testMatcher{test: p.Test}, nil
}

type testMatcher struct{ test string }

func (testMatcher) Listens() []events.Kind { return []events.Kind{events.TestPassed} }

func (m testMatcher) Advance(ev events.Event, p *Progress, max int) bool {
	return ev.Test == m.test && p.Count(1, max)
}

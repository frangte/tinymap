package directions

import "saigon-noir/server/internal/protocol"

// Server-to-client messages of asking the way (server/README.md, Asking the way).

// Person is a player on the other side of an ask.
type Person struct {
	ID   uint32 `json:"id"`
	Name string `json:"name"`
}

// States of an ask, as the asker hears them.
const (
	StateAsking     = "asking"
	StateAnswered   = "answered"
	StateUnanswered = "unanswered"
	StateCancelled  = "cancelled"
)

// wayMsg tells the asker where its ask stands.
type wayMsg struct {
	T     string `json:"t"`
	ID    string `json:"id,omitempty"`
	State string `json:"state"`
	Dest  string `json:"dest"`
	// Until is when an ask stops waiting (ms since the epoch), while asking
	Until int64 `json:"until,omitempty"`
	// Helper is who answered; Why says why it went unanswered (nobody: no Citizen online; no_answer: time ran out)
	Helper *Person `json:"helper,omitempty"`
	Why    string  `json:"why,omitempty"`
}

// callMsg asks a Citizen to come and show the way; the asker stays in their view while the call stands.
type callMsg struct {
	T     string `json:"t"`
	ID    string `json:"id"`
	From  Person `json:"from"`
	Dest  string `json:"dest"`
	Until int64  `json:"until"`
}

// goneMsg withdraws a call: taken (someone else answered), cancelled, expired, left (the asker left), changed (the
// one called is no longer a Citizen).
type goneMsg struct {
	T   string `json:"t"`
	ID  string `json:"id"`
	Why string `json:"why"`
}

// doneMsg thanks the Citizen who answered.
type doneMsg struct {
	T    string `json:"t"`
	ID   string `json:"id"`
	To   Person `json:"to"`
	Dest string `json:"dest"`
}

type errorMsg struct {
	T    string `json:"t"`
	Code string `json:"code"`
}

// Error codes (way_error).
const (
	ErrNoAccount     = "no_account"
	ErrNotAllowed    = "not_allowed"
	ErrBusy          = "busy"
	ErrUnknownPlace  = "unknown_place"
	ErrTooSoon       = "too_soon"
	ErrGone          = "gone"
	ErrTooFar        = "too_far"
	ErrAlreadyHelped = "already_helped"
)

const (
	typeWay   = "way"
	typeCall  = "way_call"
	typeGone  = "way_gone"
	typeDone  = "way_done"
	typeError = "way_error"
)

func encode(v any) []byte { return protocol.Encode(v) }

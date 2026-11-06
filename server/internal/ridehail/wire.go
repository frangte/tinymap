package ridehail

import "saigon-noir/server/internal/protocol"

// Server-to-client messages of ride-hailing (server/README.md, Crab Bike trips). They are ride-specific, so they
// live here rather than in the core protocol package.

type walletMsg struct {
	T       string `json:"t"`
	Balance int64  `json:"balance"`
}

type dutyMsg struct {
	T  string `json:"t"`
	On bool   `json:"on"`
}

// Person is someone on a trip, as the other side sees them.
type Person struct {
	ID    uint32  `json:"id"`
	Name  string  `json:"name"`
	Stars float64 `json:"stars,omitempty"`
	Trips int     `json:"trips,omitempty"`
}

type rideMsg struct {
	T     string  `json:"t"`
	ID    string  `json:"id"`
	State string  `json:"state"`
	Role  string  `json:"role"`
	Dest  string  `json:"dest"`
	Fare  int64   `json:"fare"`
	Pass  Person  `json:"passenger"`
	Drv   *Person `json:"driver,omitempty"`
	// Pickup is where the passenger booked from (three.js x, z), for the driver's guidance
	Pickup [2]float64 `json:"pickup"`
	// Deadline is when the current state times out (ms since the epoch), 0 for none
	Deadline int64 `json:"deadline,omitempty"`
	// End says how it ended (completed, alighted, cancelled_by_passenger, …) and Paid what the driver got
	End  string `json:"end,omitempty"`
	Paid int64  `json:"paid,omitempty"`
}

type offerMsg struct {
	T       string     `json:"t"`
	Offer   string     `json:"offer"`
	Dest    string     `json:"dest"`
	Fare    int64      `json:"fare"`
	Pass    Person     `json:"passenger"`
	Pickup  [2]float64 `json:"pickup"`
	Expires int64      `json:"expires"`
}

type offerGoneMsg struct {
	T     string `json:"t"`
	Offer string `json:"offer"`
	Why   string `json:"why"`
}

// ratedMsg: the passenger's rating was stored.
type ratedMsg struct {
	T  string `json:"t"`
	ID string `json:"id"`
}

type errorMsg struct {
	T    string `json:"t"`
	Code string `json:"code"`
}

// Error codes in ride_error.
const (
	ErrNoAccount    = "no_account"
	ErrBusy         = "busy"
	ErrUnknownPlace = "unknown_place"
	ErrNoBike       = "no_bike"
	ErrTooFar       = "too_far"
	ErrNotStopped   = "not_stopped"
	ErrNotThere     = "not_there"
	ErrBalanceLow   = "balance"
	ErrGone         = "gone"
	ErrNotAllowed   = "not_allowed"
	ErrTip          = "tip"
	// ErrHere: at the destination already
	ErrHere = "here"
	// ErrTooSoon: booked less than 10 s ago
	ErrTooSoon = "too_soon"
	// ErrRiding: on a vehicle already (book or board on foot)
	ErrRiding = "riding"
)

func encode(v any) []byte { return protocol.Encode(v) }

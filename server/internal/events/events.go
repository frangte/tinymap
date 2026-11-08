// Package events are the facts the world observes about players, for the systems that react to them (quests and
// achievements, server/internal/quest). The world publishes them and never waits: everything an event leads to
// (progress, rewards, the database) happens elsewhere. A client never sends an event; the world only publishes
// what it saw happen: moves it accepted, conversations it ran, emotes it showed others.
package events

// Kind says what happened; which fields of an Event are set depends on it.
type Kind string

const (
	// PlaceEntered: the player has stood in Place (of PlaceKind) for 2 s; Riding if on a vehicle by then
	PlaceEntered Kind = "place.entered"
	// PlaceTime: another Seconds spent in Place since the last one (every 5 s while there); Riding as above
	PlaceTime Kind = "place.time"
	// NPCTalked: the player started a conversation with NPC (or joined its group)
	NPCTalked Kind = "npc.talked"
	// Signal: an answer in a conversation with NPC sent Signal
	Signal Kind = "dialogue.signal"
	// Emote: the player played Emote in Place
	Emote Kind = "emote"
	// Greeted: the player and Partners (other characters) played the same Emote at each other, close by and
	// within a few seconds: a greeting, returned
	Greeted Kind = "greeted"
	// Mounted: the player got on a Vehicle (its kind)
	Mounted Kind = "vehicle.mounted"
	// Rode: the player rode another Metres
	Rode Kind = "ride.distance"
	// QuestDone: the player completed Quest (published by the quest engine for its own achievements)
	QuestDone Kind = "quest.done"
	// DailyStreak: the player has now done a daily quest on Days days in a row (published by the quest engine)
	DailyStreak Kind = "daily.streak"
	// DirectionsAnswered: a Citizen player (Partners) answered the player's ask for the way to Place
	DirectionsAnswered Kind = "directions.answered"
	// DirectionsGiven: the player gave the way to Place to Partners (the one who asked)
	DirectionsGiven Kind = "directions.given"
	// DirectionsUnanswered: nobody answered the player's ask for the way to Place in time, or nobody could
	DirectionsUnanswered Kind = "directions.unanswered"
	// TripCompleted: a Crab Bike trip the player took part in as Role ("rider", who drove, or "passenger") ended paid,
	// with Partners (the other one) over Metres
	TripCompleted Kind = "trip.completed"
)

// Event is one fact about one character.
type Event struct {
	Kind Kind
	// Character is the stored character it happened to; guests without an account have none and get no events
	Character string
	Place     string
	PlaceKind string
	NPC       string
	Signal    string
	Emote     string
	Vehicle   string
	Quest     string
	Role      string
	Riding    bool
	Seconds   int
	Metres    int
	Days      int
	// Partners are the other characters of a greeting, in id order (distinct, never the character itself)
	Partners []string
}

package weather

import (
	"bytes"
	"time"

	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/world"
)

// checkEvery is how often the module looks at the Source and for players who have not had the weather yet.
const checkEvery = time.Second

// Reader gives the weather to show now (Source).
type Reader interface {
	Now() Now
}

// Players is what the module needs of the world (a world.Host has it).
type Players interface {
	Each(fn func(world.Member))
	Send(id uint32, frame []byte)
}

// Module sends the weather: to each player once on arrival, and to everyone again when it changes. A world module:
// it runs on the world goroutine and only reads the Source.
type Module struct {
	h   Players
	src Reader
	// frame is the weather message last sent; sent who has it
	frame     []byte
	sent      map[uint32]bool
	checkedAt time.Time
}

// New makes the module; pass it to world.Use with a closure over the source.
func New(h Players, src Reader) *Module {
	return &Module{h: h, src: src, sent: map[uint32]bool{}}
}

// Frame is the message for w.
func Frame(w Now) []byte {
	msg := protocol.Weather{T: protocol.TypeWeather, Kind: string(w.Kind), Rain: w.Rain, Cloud: w.Cloud}
	if !w.At.IsZero() {
		t := w.Temp
		msg.Temp = &t
	}
	return protocol.Encode(msg)
}

// Handle: players send nothing about the weather.
func (m *Module) Handle(world.Member, protocol.ClientMsg) bool { return false }

// Tick sends the weather to whoever does not have this one yet.
func (m *Module) Tick(now time.Time) {
	if now.Sub(m.checkedAt) < checkEvery {
		return
	}
	m.checkedAt = now
	if f := Frame(m.src.Now()); !bytes.Equal(f, m.frame) {
		m.frame = f
		clear(m.sent)
	}
	m.h.Each(func(p world.Member) {
		if id := p.ID(); !m.sent[id] {
			m.sent[id] = true
			m.h.Send(id, m.frame)
		}
	})
}

// Left forgets a player, so the id's next owner gets the weather.
func (m *Module) Left(p world.Member) {
	delete(m.sent, p.ID())
}

// Command loadbots connects many walking bots to a game server and reports what each one receives, for the
// roadmap's budgets (docs/design/06-mvp-roadmap.md: phase 5's load test, 300 players).
//
//	go run ./cmd/loadbots -url ws://localhost:8787/ws -n 300 -seconds 60 -accounts -chat 1
//
// Each bot joins (as a guest, or with -accounts on a guest account of its own, so quests, pose saves and wallets
// run too), then walks the nav grid the way a player would (3.1 m/s, ten reports a second, never through a wall),
// so the server should refuse none of its moves. With -chat it says a line now and then (mostly nearby). The
// server's MAX_PLAYERS must leave room, and -accounts needs GUEST_BURST on the server (one address takes them all).
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"saigon-noir/server/content"
)

type stats struct {
	bytes, frames, corrections atomic.Int64
	// welcome is how long the hello took to answer (ms)
	welcome atomic.Int64
	mu      sync.Mutex
	// codes counts the errors received, types the frames by type
	codes map[string]int
	types map[string]int
}

func (s *stats) count(t, code string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.types[t]++
	if code != "" {
		s.codes[code]++
	}
}

type options struct {
	url, origin string
	accounts    bool
	chat        float64
	stop        time.Time
	// archs are the archetypes the bots take turns at, with accounts (archetypes.json)
	archs []string
}

func main() {
	url := flag.String("url", "ws://localhost:8787/ws", "the game socket")
	origin := flag.String("origin", "", "Origin header to send (the server allows none, or its own host)")
	n := flag.Int("n", 150, "bots")
	seconds := flag.Int("seconds", 30, "how long to walk")
	accounts := flag.Bool("accounts", false, "take a guest account each (POST /api/auth/guest; the server needs GUEST_BURST)")
	chat := flag.Float64("chat", 0, "chat lines per bot per minute (0: none)")
	flag.Parse()
	grid, err := content.Nav()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	catalog, err := content.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	opt := options{url: *url, origin: *origin, accounts: *accounts, chat: *chat, stop: time.Now().Add(time.Duration(*seconds) * time.Second)}
	for _, a := range catalog.Archetypes {
		opt.archs = append(opt.archs, a.ID)
	}
	bots := make([]*stats, *n)
	var wg sync.WaitGroup
	var failed atomic.Int64
	for i := range bots {
		bots[i] = &stats{codes: map[string]int{}, types: map[string]int{}}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := run(opt, grid, bots[i], uint64(i)); err != nil {
				failed.Add(1)
				fmt.Fprintf(os.Stderr, "bot %d: %v\n", i, err)
			}
		}(i)
		time.Sleep(20 * time.Millisecond) // do not arrive all in one millisecond
	}
	wg.Wait()
	report(bots, float64(*seconds), failed.Load())
}

func report(bots []*stats, seconds float64, failed int64) {
	rates := make([]float64, 0, len(bots))
	var welcomes []int64
	var corrections, frames int64
	codes, types := map[string]int{}, map[string]int{}
	for _, s := range bots {
		rates = append(rates, float64(s.bytes.Load())/seconds/1000)
		corrections += s.corrections.Load()
		frames += s.frames.Load()
		if w := s.welcome.Load(); w > 0 {
			welcomes = append(welcomes, w)
		}
		for k, v := range s.codes {
			codes[k] += v
		}
		for k, v := range s.types {
			types[k] += v
		}
	}
	sort.Float64s(rates)
	sort.Slice(welcomes, func(i, j int) bool { return welcomes[i] < welcomes[j] })
	sum := 0.0
	for _, r := range rates {
		sum += r
	}
	fmt.Printf("%d bots for %.0f s (%d failed): received %.1f KB/s on average, %.1f median, %.1f at most; %.0f frames/s each; %d moves refused\n",
		len(bots), seconds, failed, sum/float64(len(rates)), rates[len(rates)/2], rates[len(rates)-1],
		float64(frames)/seconds/float64(len(bots)), corrections)
	if len(welcomes) > 0 {
		fmt.Printf("welcome after %d ms (median), %d ms (95th), %d ms (slowest)\n",
			welcomes[len(welcomes)/2], welcomes[len(welcomes)*95/100], welcomes[len(welcomes)-1])
	}
	fmt.Printf("frames by type: %s\n", counts(types))
	if len(codes) > 0 {
		fmt.Printf("errors: %s\n", counts(codes))
	}
}

func counts(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return m[keys[i]] > m[keys[j]] })
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s %d", k, m[k])
	}
	return strings.Join(parts, ", ")
}

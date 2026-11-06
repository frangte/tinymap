// Command loadbots connects many walking bots to a game server and reports what each one receives, for the
// roadmap's budgets (docs/design/06-mvp-roadmap.md: 150 bots at 12 Hz under ~25 KB/s per client).
//
//	go run ./cmd/loadbots -url ws://localhost:8787/ws -n 150 -seconds 30
//
// Each bot joins as a guest, then walks the nav grid the way a player would (3.1 m/s, ten reports a second,
// never through a wall), so the server should refuse none of its moves. The server's MAX_PLAYERS must leave room.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"saigon-noir/server/content"
	"saigon-noir/server/internal/nav"
)

const (
	walkSpeed  = 3.1
	reportsPer = 10 // a second
	bodyRadius = 0.3
)

type stats struct {
	bytes, frames, corrections atomic.Int64
}

func main() {
	url := flag.String("url", "ws://localhost:8787/ws", "the game socket")
	origin := flag.String("origin", "", "Origin header to send (the server allows none, or its own host)")
	n := flag.Int("n", 150, "bots")
	seconds := flag.Int("seconds", 30, "how long to walk")
	flag.Parse()
	grid, err := content.Nav()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	bots := make([]*stats, *n)
	var wg sync.WaitGroup
	var failed atomic.Int64
	stop := time.Now().Add(time.Duration(*seconds) * time.Second)
	for i := range bots {
		bots[i] = &stats{}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := run(*url, *origin, grid, bots[i], stop, uint64(i)); err != nil {
				failed.Add(1)
				fmt.Fprintf(os.Stderr, "bot %d: %v\n", i, err)
			}
		}(i)
		time.Sleep(20 * time.Millisecond) // do not arrive all in one millisecond
	}
	wg.Wait()
	report(bots, float64(*seconds), failed.Load())
}

// run is one bot: hello, then walk until stop while a reader counts what arrives.
func run(url, origin string, grid *nav.Grid, s *stats, stop time.Time, seed uint64) error {
	hdr := http.Header{}
	if origin != "" {
		hdr.Set("Origin", origin)
	}
	conn, resp, err := websocket.DefaultDialer.Dial(url, hdr)
	if err != nil {
		return err
	}
	resp.Body.Close()
	defer conn.Close()
	hello := fmt.Sprintf(`{"t":"hello","name":"bot %d","look":{"body":"man","outfit":"thun","hat":"cap"}}`, seed)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(hello)); err != nil {
		return err
	}
	var welcome struct {
		T    string     `json:"t"`
		Code string     `json:"code"`
		Pose [4]float64 `json:"pose"`
	}
	_, data, err := conn.ReadMessage()
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &welcome); err != nil || welcome.T != "welcome" {
		return fmt.Errorf("no welcome: %s", data)
	}

	var mu sync.Mutex
	x, y, z := welcome.Pose[0], welcome.Pose[1], welcome.Pose[2]
	var k uint32
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, b, err := conn.ReadMessage()
			if err != nil {
				return
			}
			s.bytes.Add(int64(len(b)))
			s.frames.Add(1)
			var c struct {
				T    string     `json:"t"`
				K    uint32     `json:"k"`
				Pose [4]float64 `json:"pose"`
			}
			if json.Unmarshal(b, &c) == nil && c.T == "correct" {
				s.corrections.Add(1)
				mu.Lock()
				x, y, z, k = c.Pose[0], c.Pose[1], c.Pose[2], c.K
				mu.Unlock()
			}
		}
	}()

	rng := rand.New(rand.NewPCG(seed, 7))
	heading := rng.Float64() * 2 * math.Pi
	tick := time.NewTicker(time.Second / reportsPer)
	defer tick.Stop()
	step := walkSpeed / reportsPer
	for time.Now().Before(stop) {
		select {
		case <-tick.C:
		case <-done:
			return fmt.Errorf("the server closed the connection")
		}
		mu.Lock()
		for try := 0; try < 8; try++ {
			nx, nz := x+math.Sin(heading)*step, z+math.Cos(heading)*step
			if grid.Fits(nx, nz, bodyRadius) && grid.Clear(x, z, nx, nz, 0) && smallStep(grid, x, z, nx, nz) {
				x, z = nx, nz
				y, _ = grid.Floor(x, z)
				break
			}
			heading = rng.Float64() * 2 * math.Pi
		}
		msg := fmt.Sprintf(`{"t":"state","x":%.2f,"y":%.2f,"z":%.2f,"h":%.3f,"s":0,"k":%d}`, x, y, z, heading, k)
		mu.Unlock()
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			return err
		}
	}
	return nil
}

// smallStep keeps bots off ledges the game would not walk off either.
func smallStep(grid *nav.Grid, ax, az, bx, bz float64) bool {
	fa, _ := grid.Floor(ax, az)
	fb, _ := grid.Floor(bx, bz)
	return math.Abs(fa-fb) <= 0.4
}

func report(bots []*stats, seconds float64, failed int64) {
	rates := make([]float64, 0, len(bots))
	var corrections, frames int64
	for _, s := range bots {
		rates = append(rates, float64(s.bytes.Load())/seconds/1000)
		corrections += s.corrections.Load()
		frames += s.frames.Load()
	}
	sort.Float64s(rates)
	sum := 0.0
	for _, r := range rates {
		sum += r
	}
	fmt.Printf("%d bots for %.0f s (%d failed): received %.1f KB/s on average, %.1f median, %.1f at most; %.0f frames/s each; %d moves refused\n",
		len(bots), seconds, failed, sum/float64(len(rates)), rates[len(rates)/2], rates[len(rates)-1],
		float64(frames)/seconds/float64(len(bots)), corrections)
}

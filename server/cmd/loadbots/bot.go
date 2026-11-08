package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"saigon-noir/server/internal/nav"
)

const (
	walkSpeed  = 3.1
	reportsPer = 10 // a second
	bodyRadius = 0.3
)

var lines = []string{"chào mọi người", "ai đi cà phê không", "đường nào ra Bến Thành vậy", "nắng quá", "hello Sài Gòn", "đẹp ghê"}

// guestToken takes a guest account, as the game does on a first visit.
func guestToken(wsURL, origin string) (string, error) {
	httpURL := strings.Replace(strings.TrimSuffix(wsURL, "/ws"), "ws", "http", 1) + "/api/auth/guest"
	req, err := http.NewRequest(http.MethodPost, httpURL, nil)
	if err != nil {
		return "", err
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || resp.StatusCode != http.StatusOK || body.Token == "" {
		return "", fmt.Errorf("guest account: %s", resp.Status)
	}
	return body.Token, nil
}

// run is one bot: hello, then walk until stop while a reader counts what arrives.
func run(opt options, grid *nav.Grid, s *stats, seed uint64) error {
	hdr := http.Header{}
	if opt.origin != "" {
		hdr.Set("Origin", opt.origin)
	}
	token, arch := "", ""
	if opt.accounts {
		var err error
		if token, err = guestToken(opt.url, opt.origin); err != nil {
			return err
		}
		if len(opt.archs) > 0 {
			arch = opt.archs[seed%uint64(len(opt.archs))]
		}
	}
	start := time.Now()
	conn, resp, err := websocket.DefaultDialer.Dial(opt.url, hdr)
	if err != nil {
		return err
	}
	resp.Body.Close()
	defer conn.Close()
	hello, _ := json.Marshal(map[string]any{"t": "hello", "token": token, "arch": arch, "name": fmt.Sprintf("bot %d", seed),
		"look": map[string]string{"body": "man", "outfit": "thun", "hat": "cap"}})
	if err := conn.WriteMessage(websocket.TextMessage, hello); err != nil {
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
	s.welcome.Store(max(1, time.Since(start).Milliseconds()))

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
				Code string     `json:"code"`
				K    uint32     `json:"k"`
				Pose [4]float64 `json:"pose"`
			}
			if json.Unmarshal(b, &c) != nil {
				continue
			}
			s.count(c.T, c.Code)
			if c.T == "correct" {
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
	// a line now and then: chat per minute, spread over the reports
	chatChance := opt.chat / 60 / reportsPer
	for time.Now().Before(opt.stop) {
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
		if chatChance > 0 && rng.Float64() < chatChance {
			ch := "nearby"
			if rng.IntN(4) == 0 {
				ch = "global"
			}
			line, _ := json.Marshal(map[string]string{"t": "chat", "ch": ch, "text": lines[rng.IntN(len(lines))]})
			if err := conn.WriteMessage(websocket.TextMessage, line); err != nil {
				return err
			}
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

package weather

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

// DefaultURL asks Open-Meteo for the weather now over District 1. Its free plan is for non-commercial use; a paid
// plan or a self-hosted Open-Meteo takes the same query (WEATHER_URL).
const DefaultURL = "https://api.open-meteo.com/v1/forecast?latitude=10.7725&longitude=106.698" +
	"&current=weather_code,cloud_cover,precipitation,temperature_2m&timezone=Asia%2FHo_Chi_Minh"

const (
	// Every is how often Source asks: Open-Meteo's current weather moves in 15-minute steps
	Every = 15 * time.Minute
	// Stale is how old the last good report may get (the API down for a while) before the sky goes back to clear
	Stale = 90 * time.Minute
	// fetchTimeout bounds one request
	fetchTimeout = 10 * time.Second
	// maxBody is more than an answer ever needs
	maxBody = 64 << 10
)

// Source fetches the weather in the background and keeps the last good report. Safe for concurrent use.
type Source struct {
	url    string
	client *http.Client
	log    *slog.Logger
	// now is the clock (tests replace it)
	now  func() time.Time
	last atomic.Pointer[stamped]
}

type stamped struct {
	w       Now
	fetched time.Time
}

// NewSource asks url (DefaultURL for Open-Meteo).
func NewSource(url string, log *slog.Logger) *Source {
	return &Source{url: url, client: &http.Client{Timeout: fetchTimeout}, log: log, now: time.Now}
}

// Run fetches at once, then every Every, until ctx ends.
func (s *Source) Run(ctx context.Context) {
	t := time.NewTicker(Every)
	defer t.Stop()
	for {
		if err := s.Fetch(ctx); err != nil && ctx.Err() == nil {
			s.log.Warn("weather: keeping the last report", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Fetch asks once and keeps the answer; on a failure the last report stays.
func (s *Source) Fetch(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return fmt.Errorf("weather request: %w", err)
	}
	res, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("weather request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("weather request: HTTP %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxBody))
	if err != nil {
		return fmt.Errorf("weather request: %w", err)
	}
	w, err := parse(body)
	if err != nil {
		return err
	}
	s.last.Store(&stamped{w: w, fetched: s.now()})
	return nil
}

// Now is the weather to show: the last report while it is fresh, else a clear sky.
func (s *Source) Now() Now {
	l := s.last.Load()
	if l == nil || s.now().Sub(l.fetched) > Stale {
		return ClearSky
	}
	return l.w
}

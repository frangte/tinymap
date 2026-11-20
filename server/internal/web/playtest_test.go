package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"saigon-noir/server/internal/playtest"
	"saigon-noir/server/internal/world"
)

type fakePlaytester struct {
	calls int
	err   error
}

func (f *fakePlaytester) Make(_ context.Context, req playtest.Request) (playtest.Result, error) {
	f.calls++
	if f.err != nil {
		return playtest.Result{}, f.err
	}
	return playtest.Result{Token: "tok", Archetype: req.Archetype}, nil
}

func playtestEngine(pt Playtester) *gin.Engine {
	r := gin.New()
	playtestRoute(r.Group("/api"), pt)
	return r
}

func post(r *gin.Engine, remote, body string, header http.Header) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/dev/playtest", strings.NewReader(body))
	req.RemoteAddr = remote
	req.Header.Set("Content-Type", "application/json")
	for k, v := range header {
		req.Header[k] = v
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestPlaytestRouteDoesNotExistByDefault(t *testing.T) {
	s := startServer(t, world.DefaultConfig())
	res, err := http.Post(s.URL+"/api/dev/playtest", "application/json", strings.NewReader(`{"archetype":"citizen"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d, want 404 with the dev tools off", res.StatusCode)
	}
}

func TestPlaytestRouteOnForLoopback(t *testing.T) {
	fake := &fakePlaytester{}
	s := startWith(t, world.DefaultConfig(), Options{Playtest: fake})
	res, err := http.Post(s.URL+"/api/dev/playtest", "application/json", strings.NewReader(`{"archetype":"citizen"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || fake.calls != 1 {
		t.Fatalf("status %d, %d calls", res.StatusCode, fake.calls)
	}
}

func TestPlaytestRouteGates(t *testing.T) {
	cases := []struct {
		name   string
		remote string
		header http.Header
		want   int
	}{
		{"loopback v4", "127.0.0.1:5555", nil, http.StatusOK},
		{"loopback v6", "[::1]:5555", nil, http.StatusOK},
		{"a LAN address", "192.168.1.20:5555", nil, http.StatusForbidden},
		{"a public address", "203.0.113.9:5555", nil, http.StatusForbidden},
		{"loopback through a reverse proxy", "127.0.0.1:5555", http.Header{"X-Forwarded-For": {"203.0.113.9"}}, http.StatusForbidden},
		{"claiming to be local does not help", "203.0.113.9:5555", http.Header{"X-Forwarded-For": {"127.0.0.1"}}, http.StatusForbidden},
		{"a forwarded host", "127.0.0.1:5555", http.Header{"X-Forwarded-Host": {"sg.example"}}, http.StatusForbidden},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &fakePlaytester{}
			rec := post(playtestEngine(fake), c.remote, `{"archetype":"citizen"}`, c.header)
			if rec.Code != c.want {
				t.Fatalf("status %d, want %d (%s)", rec.Code, c.want, rec.Body)
			}
			if c.want != http.StatusOK && fake.calls != 0 {
				t.Fatal("a refused caller reached the maker")
			}
		})
	}
}

func TestPlaytestRouteRefusals(t *testing.T) {
	cases := []struct {
		name string
		body string
		err  error
		want int
	}{
		{"not json", `{"archetype"`, nil, http.StatusBadRequest},
		{"an unknown field", `{"archetype":"citizen","admin":true}`, nil, http.StatusOK},
		{"a request the content refuses", `{"archetype":"king"}`, playtest.ErrInvalid, http.StatusBadRequest},
		{"a store failure", `{"archetype":"citizen"}`, errors.New("disk full"), http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := post(playtestEngine(&fakePlaytester{err: c.err}), "127.0.0.1:1", c.body, nil)
			if rec.Code != c.want {
				t.Fatalf("status %d, want %d (%s)", rec.Code, c.want, rec.Body)
			}
		})
	}
}

package identity_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"saigon-noir/server/internal/identity"
	"saigon-noir/server/internal/protocol"
	"saigon-noir/server/internal/store"
)

var (
	lanLook  = protocol.Look{Body: "woman", Outfit: "aodai", Hat: "nonla"}
	xeomLook = protocol.Look{Body: "woman", Outfit: "xeom", Hat: "none"}
)

func openDB(t *testing.T, path string) *store.DB {
	t.Helper()
	db, err := store.Open(context.Background(), store.Options{Path: path}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func lan(arch string) identity.Profile {
	return identity.Profile{Name: "Lan", Look: lanLook, Archetype: arch}
}

func TestGuestKeepsItsCharacterAcrossRestarts(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "game.db")
	db := openDB(t, path)
	svc := identity.New(db)

	token, err := svc.NewGuest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 43 || len(token) > protocol.TokenMax {
		t.Fatalf("token %q: want 43 characters", token)
	}
	first, _, err := svc.Arrive(ctx, token, lan(""))
	if err != nil || first == "" {
		t.Fatalf("first arrival = %q, %v", first, err)
	}
	if again, _, err := svc.Arrive(ctx, token, lan("")); err != nil || again != first {
		t.Fatalf("second arrival = %q, %v; want %q", again, err, first)
	}
	if err := svc.SaveProfile(ctx, first, "Lan Anh", xeomLook); err != nil {
		t.Fatal(err)
	}
	db.Close()

	// the server restarts
	db = openDB(t, path)
	svc = identity.New(db)
	if after, _, err := svc.Arrive(ctx, token, identity.Profile{Look: xeomLook}); err != nil || after != first {
		t.Fatalf("after a restart = %q, %v; want %q", after, err, first)
	}

	// only the token's hash is stored
	sum := sha256.Sum256([]byte(token))
	account, err := db.AccountByToken(ctx, sum[:], 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.AccountByToken(ctx, []byte(token), 0); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("the plain token must not be stored: %v", err)
	}
	c, err := db.CharacterOf(ctx, account)
	if err != nil || c.ID != first || c.Name != "Lan Anh" || c.Look != xeomLook {
		t.Fatalf("stored character = %+v, %v", c, err)
	}
}

func TestArchetypeIsChosenOnce(t *testing.T) {
	ctx := context.Background()
	svc := identity.New(openDB(t, filepath.Join(t.TempDir(), "game.db")))
	cases := []struct {
		name     string
		asks     string
		wantArch string
	}{
		{"a character from before archetypes has none", "", ""},
		{"its first choice is kept", "traveler", "traveler"},
		{"a later hello cannot change it", "driver", "traveler"},
		{"nor can a hello without one", "", "traveler"},
	}
	token, err := svc.NewGuest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, arch, err := svc.Arrive(ctx, token, lan(c.asks))
			if err != nil || arch != c.wantArch {
				t.Fatalf("archetype = %q, %v; want %q", arch, err, c.wantArch)
			}
		})
	}
	// a new account takes the archetype of its first hello
	other, _ := svc.NewGuest(ctx)
	if _, arch, err := svc.Arrive(ctx, other, lan("citizen")); err != nil || arch != "citizen" {
		t.Fatalf("new character = %q, %v", arch, err)
	}
}

func TestArrive(t *testing.T) {
	ctx := context.Background()
	svc := identity.New(openDB(t, filepath.Join(t.TempDir(), "game.db")))
	cases := []struct {
		name     string
		token    string
		wantID   bool
		wantArch string
		wantErr  error
	}{
		{"no token is a guest without an account, with the hello's archetype", "", false, "driver", nil},
		{"unknown token", "not-a-token-anyone-was-given", false, "", identity.ErrUnknownToken},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, arch, err := svc.Arrive(ctx, c.token, lan("driver"))
			if !errors.Is(err, c.wantErr) || (id != "") != c.wantID || arch != c.wantArch {
				t.Fatalf("Arrive = %q, %q, %v", id, arch, err)
			}
		})
	}
}

func TestGuestsAreSeparate(t *testing.T) {
	ctx := context.Background()
	svc := identity.New(openDB(t, filepath.Join(t.TempDir(), "game.db")))
	a, err := svc.NewGuest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.NewGuest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("two guests got the same token")
	}
	ca, _, _ := svc.Arrive(ctx, a, lan(""))
	cb, _, _ := svc.Arrive(ctx, b, identity.Profile{Name: "Minh", Look: lanLook})
	if ca == "" || ca == cb {
		t.Fatalf("characters %q and %q should differ", ca, cb)
	}
}

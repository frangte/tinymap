// Package identity is who a player is from one visit to the next: an account (for now an anonymous guest,
// known by a token the browser keeps) and its character (name and look).
//
// A connection resolves its account on its own goroutine, before it joins the world, so the world loop never
// waits on the database. In this first version the profile the client sends is what gets stored; the server
// becomes the source of truth for the character with character creation (docs/design/06-mvp-roadmap.md, phase 1).
package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"saigon-noir/server/internal/ids"
	"saigon-noir/server/internal/protocol"
)

var (
	// ErrNotFound is what a Repo returns for a missing account, token or character.
	ErrNotFound = errors.New("identity: not found")
	// ErrUnknownToken: a hello's token belongs to no account.
	ErrUnknownToken = errors.New("identity: unknown token")
)

// tokenBytes of randomness make a 43-character base64url token (protocol.TokenMax allows up to 64).
const tokenBytes = 32

// Character is a player's persistent self.
type Character struct {
	ID        string
	AccountID string
	Name      string
	Look      protocol.Look
	// Archetype is the archetype id, "" until the player has chosen one; then it stays
	Archetype string
	// Pose is where the character stood when its player last left, nil before that
	Pose *protocol.Pose
	// Title is the title it carries ("" for none); Caps the capabilities it earned while playing
	Title string
	Caps  []string
	// CreatedAt is unix milliseconds
	CreatedAt int64
}

// Arrived is the character behind a hello: its id ("" for a guest without an account), its archetype and where
// it stood when its player last left (nil for none).
type Arrived struct {
	ID        string
	Archetype string
	Pose      *protocol.Pose
	Title     string
	// Caps are the capabilities it earned while playing (quest rewards), beyond its archetype's
	Caps []string
}

// Profile is what a hello says about the player. Archetype must already be a known id, or "".
type Profile struct {
	Name      string
	Look      protocol.Look
	Archetype string
}

// Repo is the storage identity needs; package store implements it. Missing rows are ErrNotFound.
type Repo interface {
	CreateAccount(ctx context.Context, accountID string, tokenHash []byte, at int64) error
	AccountByToken(ctx context.Context, tokenHash []byte, at int64) (accountID string, err error)
	CharacterOf(ctx context.Context, accountID string) (Character, error)
	CreateCharacter(ctx context.Context, c Character) error
	UpdateProfile(ctx context.Context, characterID, name string, look protocol.Look, at int64) error
	// SetArchetype sets the archetype of a character that has none yet, and leaves one that has alone.
	SetArchetype(ctx context.Context, characterID, archetype string, at int64) error
	SavePose(ctx context.Context, characterID string, p protocol.Pose, at int64) error
}

type Service struct {
	repo Repo
	now  func() time.Time
}

func New(repo Repo) *Service { return &Service{repo: repo, now: time.Now} }

// NewGuest creates an anonymous account and returns its token. The token is shown only this once: the database
// keeps its SHA-256, so a leaked database does not let anyone sign in as a player.
func (s *Service) NewGuest(ctx context.Context) (string, error) {
	raw := make([]byte, tokenBytes)
	_, _ = rand.Read(raw) // crypto/rand.Read never fails: it crashes the program instead
	token := base64.RawURLEncoding.EncodeToString(raw)
	if err := s.repo.CreateAccount(ctx, ids.New(), hashToken(token), s.now().UnixMilli()); err != nil {
		return "", fmt.Errorf("identity: new guest: %w", err)
	}
	return token, nil
}

// Arrive finds the character behind a hello's token, creating it on the account's first visit, and stores the
// name and look the hello came with. An archetype is chosen once, so a character that has one keeps it whatever
// the hello says. An empty token is a guest without an account: no id, and the hello's archetype for this visit.
func (s *Service) Arrive(ctx context.Context, token string, p Profile) (Arrived, error) {
	if token == "" {
		return Arrived{Archetype: p.Archetype}, nil
	}
	now := s.now().UnixMilli()
	account, err := s.repo.AccountByToken(ctx, hashToken(token), now)
	if errors.Is(err, ErrNotFound) {
		return Arrived{}, ErrUnknownToken
	}
	if err != nil {
		return Arrived{}, fmt.Errorf("identity: arrive: %w", err)
	}
	c, err := s.repo.CharacterOf(ctx, account)
	if errors.Is(err, ErrNotFound) {
		c = Character{ID: ids.New(), AccountID: account, Name: p.Name, Look: p.Look, Archetype: p.Archetype, CreatedAt: now}
		if err := s.repo.CreateCharacter(ctx, c); err != nil {
			return Arrived{}, fmt.Errorf("identity: arrive: %w", err)
		}
		return Arrived{ID: c.ID, Archetype: c.Archetype}, nil
	}
	if err != nil {
		return Arrived{}, fmt.Errorf("identity: arrive: %w", err)
	}
	if c.Archetype == "" && p.Archetype != "" {
		if err := s.repo.SetArchetype(ctx, c.ID, p.Archetype, now); err != nil {
			return Arrived{}, fmt.Errorf("identity: arrive: %w", err)
		}
		c.Archetype = p.Archetype
	}
	if (p.Name != "" && p.Name != c.Name) || p.Look != c.Look {
		if err := s.repo.UpdateProfile(ctx, c.ID, p.Name, p.Look, now); err != nil {
			return Arrived{}, fmt.Errorf("identity: arrive: %w", err)
		}
	}
	return Arrived{ID: c.ID, Archetype: c.Archetype, Pose: c.Pose, Title: c.Title, Caps: c.Caps}, nil
}

// SavePose stores where the character stands, for its next visit.
func (s *Service) SavePose(ctx context.Context, characterID string, p protocol.Pose) error {
	if err := s.repo.SavePose(ctx, characterID, p, s.now().UnixMilli()); err != nil {
		return fmt.Errorf("identity: save pose: %w", err)
	}
	return nil
}

// SaveProfile stores a changed look and name (an empty name keeps the stored one).
func (s *Service) SaveProfile(ctx context.Context, characterID, name string, look protocol.Look) error {
	if err := s.repo.UpdateProfile(ctx, characterID, name, look, s.now().UnixMilli()); err != nil {
		return fmt.Errorf("identity: save profile: %w", err)
	}
	return nil
}

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

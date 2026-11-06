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
	// CreatedAt is unix milliseconds
	CreatedAt int64
}

// Repo is the storage identity needs; package store implements it. Missing rows are ErrNotFound.
type Repo interface {
	CreateAccount(ctx context.Context, accountID string, tokenHash []byte, at int64) error
	AccountByToken(ctx context.Context, tokenHash []byte, at int64) (accountID string, err error)
	CharacterOf(ctx context.Context, accountID string) (Character, error)
	CreateCharacter(ctx context.Context, c Character) error
	UpdateProfile(ctx context.Context, characterID, name string, look protocol.Look, at int64) error
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
// name and look the hello came with. An empty token is a guest without an account: "" and no error.
func (s *Service) Arrive(ctx context.Context, token, name string, look protocol.Look) (string, error) {
	if token == "" {
		return "", nil
	}
	now := s.now().UnixMilli()
	account, err := s.repo.AccountByToken(ctx, hashToken(token), now)
	if errors.Is(err, ErrNotFound) {
		return "", ErrUnknownToken
	}
	if err != nil {
		return "", fmt.Errorf("identity: arrive: %w", err)
	}
	c, err := s.repo.CharacterOf(ctx, account)
	if errors.Is(err, ErrNotFound) {
		c = Character{ID: ids.New(), AccountID: account, Name: name, Look: look, CreatedAt: now}
		if err := s.repo.CreateCharacter(ctx, c); err != nil {
			return "", fmt.Errorf("identity: arrive: %w", err)
		}
		return c.ID, nil
	}
	if err != nil {
		return "", fmt.Errorf("identity: arrive: %w", err)
	}
	if (name != "" && name != c.Name) || look != c.Look {
		if err := s.repo.UpdateProfile(ctx, c.ID, name, look, now); err != nil {
			return "", fmt.Errorf("identity: arrive: %w", err)
		}
	}
	return c.ID, nil
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

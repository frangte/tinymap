package shop

import (
	"context"
	"errors"
)

// Purchase is one item bought by one character.
type Purchase struct {
	Character string
	Item      string
	// Price is in xu; Grants are the capabilities the item gives (stored with source "item:<Item>")
	Price  int64
	Grants []string
	// At is when it happened (ms since the epoch)
	At int64
}

// Payment is one fee paid by one character (docs/design/13-driver.md).
type Payment struct {
	Character string
	Fee       string
	// Amount is in xu; Grants are the capabilities paying gives (stored with source "fee:<Fee>")
	Amount int64
	Grants []string
	At     int64
}

// Errors a Bank answers a purchase with.
var (
	// ErrBalance: the wallet does not cover the price; nothing was kept
	ErrBalance = errors.New("shop: balance too low")
	// ErrOwned: the character has the item already (a retried purchase lands here too); nothing moved
	ErrOwned = errors.New("shop: already owned")
)

// Bank keeps what characters own and moves their money (store.DB in production).
type Bank interface {
	// Buy takes the price from the wallet and records the item and its capability grants, in one transaction;
	// it returns the new balance. ErrOwned and ErrBalance leave everything as it was.
	Buy(ctx context.Context, p Purchase) (balance int64, err error)
	// Pay takes a fee from the wallet, records the capabilities it grants and a ledger line with the reason 'fee', in
	// one transaction; it returns the new balance. ErrOwned (the character holds what paying would give) and
	// ErrBalance leave everything as it was.
	Pay(ctx context.Context, p Payment) (balance int64, err error)
	// Owned lists the ids of the items a character has, in the order it got them.
	Owned(ctx context.Context, character string) ([]string, error)
}

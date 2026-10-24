// Package system provides the real clock and run IDs behind the use-case ports.
package system

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Clock implements usecase.Clock with the wall clock.
type Clock struct{}

func (Clock) Now() time.Time { return time.Now() }

// IDs implements usecase.IDGenerator: "r_" and 16 random hex characters.
type IDs struct{}

func (IDs) NewID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b) // crypto/rand.Read never fails on supported platforms
	return "r_" + hex.EncodeToString(b)
}

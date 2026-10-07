package logger

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// NewID returns a short random hex string, used for both job IDs and
// execution IDs — anything that needs a cheap, effectively-unique
// correlation ID without pulling in a UUID dependency.
func NewID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing is effectively unheard of on real systems;
		// fall back to a timestamp so callers never have to handle an error.
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(b)
}

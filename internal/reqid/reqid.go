// Package reqid names each request, so its log line can be found from a report of it.
package reqid

import (
	"crypto/rand"
	"encoding/hex"
)

// New returns a random 16-character id.
func New() string {
	var b [8]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

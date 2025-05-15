package protocol

import (
	crand "crypto/rand"
	"encoding/binary"
	mrand "math/rand"
	"time"
)

// GeneratePairingKey returns a cryptographically random 32-bit key.
// If crypto/rand fails (rare on host), falls back to math/rand.
func GeneratePairingKey() uint32 {
	var b [4]byte
	if _, err := crand.Read(b[:]); err == nil {
		return binary.LittleEndian.Uint32(b[:])
	}
	src := mrand.NewSource(time.Now().UnixNano())
	return mrand.New(src).Uint32()
}

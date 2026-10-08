package ws

import (
	"crypto/rand"
	"encoding/hex"
)

func randomPeerID() string {
	var value [6]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic("crypto/rand unavailable")
	}
	return "c_" + hex.EncodeToString(value[:])
}

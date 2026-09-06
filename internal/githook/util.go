package githook

import (
	"crypto/sha256"
	"encoding/hex"
)

func sha256Hex(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

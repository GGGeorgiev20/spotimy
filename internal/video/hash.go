package video

import (
	"crypto/sha256"
	"encoding/hex"
)

func hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:16]) }

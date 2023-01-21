package main

import (
	"crypto/sha1"
	"encoding/hex"
)

func GetHash(s string) string {
	h := sha1.New()
	h.Write([]byte(s))

	return hex.EncodeToString(h.Sum(nil))
}

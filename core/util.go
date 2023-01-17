package core

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
)

func GetTempDir(session *Session, suffix string) string {
	dir := filepath.Join(*session.Options.TempDirectory, suffix)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, os.ModePerm)
	} else {
		os.RemoveAll(dir)
	}

	return dir
}

func GetHash(s string) string {
	h := sha1.New()
	h.Write([]byte(s))

	return hex.EncodeToString(h.Sum(nil))
}

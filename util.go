package main

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
)

func GetTempDir(suffix string) (string, error) {
	dir := filepath.Join("/tmp", suffix)

	_, err := os.Stat(dir)
	if !os.IsNotExist(err) {
		os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}

func GetHash(s string) string {
	h := sha1.New()
	h.Write([]byte(s))

	return hex.EncodeToString(h.Sum(nil))
}

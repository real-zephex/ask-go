package main

import (
	"crypto/sha256"
	"fmt"
)

func hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}

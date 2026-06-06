// Small one-off helper for the Phase 3 smoke test.
// Reads ENCRYPTION_KEY from the environment, encrypts the first CLI arg, prints the ciphertext.
package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/rabb1tof/socialsentry/backend/pkg/crypto"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: encrypt-token <plaintext>")
		os.Exit(1)
	}
	keyHex := os.Getenv("ENCRYPTION_KEY")
	if keyHex == "" {
		fmt.Fprintln(os.Stderr, "ENCRYPTION_KEY env var is required")
		os.Exit(1)
	}
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bad hex key:", err)
		os.Exit(1)
	}
	enc, err := crypto.Encrypt(os.Args[1], key)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(enc)
}

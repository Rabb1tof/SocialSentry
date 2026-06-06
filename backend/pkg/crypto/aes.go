// Package crypto provides AES-256-GCM encryption for platform tokens stored in the database.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// Encrypt encrypts plaintext using AES-256-GCM with the given 32-byte key.
// The nonce is prepended to the ciphertext; the result is base64-encoded.
func Encrypt(plaintext string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("crypto.Encrypt: key must be 32 bytes, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("crypto.Encrypt: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto.Encrypt: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto.Encrypt: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt reverses Encrypt: base64-decodes, extracts the nonce, and returns the plaintext.
func Decrypt(encoded string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("crypto.Decrypt: key must be 32 bytes, got %d", len(key))
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("crypto.Decrypt: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("crypto.Decrypt: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto.Decrypt: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("crypto.Decrypt: ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("crypto.Decrypt: %w", err)
	}

	return string(plaintext), nil
}

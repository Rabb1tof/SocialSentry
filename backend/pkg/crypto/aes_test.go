package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand: %v", err)
	}

	cases := []struct {
		name      string
		plaintext string
	}{
		{"empty", ""},
		{"short", "hello"},
		{"long", "the quick brown fox jumps over the lazy dog " +
			"the quick brown fox jumps over the lazy dog"},
		{"unicode", "Привет, мир! 🚀"},
		{"token-like", "EAAZAjwUlxPMBOZBHKZA8jwUhU9YjwU"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			enc, err := Encrypt(tc.plaintext, key)
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}
			if tc.plaintext != "" && enc == tc.plaintext {
				t.Fatalf("ciphertext equals plaintext")
			}
			dec, err := Decrypt(enc, key)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}
			if dec != tc.plaintext {
				t.Fatalf("got %q, want %q", dec, tc.plaintext)
			}
		})
	}
}

func TestEncrypt_KeyTooShort(t *testing.T) {
	if _, err := Encrypt("data", make([]byte, 16)); err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := bytes.Repeat([]byte{1}, 32)
	key2 := bytes.Repeat([]byte{2}, 32)

	enc, err := Encrypt("secret", key1)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if _, err := Decrypt(enc, key2); err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestDecrypt_Tampered(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)
	enc, err := Encrypt("secret payload", key)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	bs := []byte(enc)
	bs[len(bs)-3] ^= 0xFF
	if _, err := Decrypt(string(bs), key); err == nil {
		t.Fatal("expected error for tampered ciphertext")
	}
}

func TestDecrypt_TooShort(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)
	// base64("hi") = "aGk=" — far too short to contain a GCM nonce
	if _, err := Decrypt("aGk=", key); err == nil {
		t.Fatal("expected error for too-short ciphertext")
	}
}

func TestDecrypt_InvalidBase64(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)
	if _, err := Decrypt("!!!not-base64!!!", key); err == nil {
		t.Fatal("expected error for non-base64 input")
	}
}

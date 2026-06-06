package jwt

import (
	"strings"
	"testing"
	"time"
)

var testSecret = []byte("a-test-secret-32-bytes-long-1234")

func TestGenerateAndParse(t *testing.T) {
	token, err := Generate("user-123", "user", time.Hour, testSecret)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if strings.Count(token, ".") != 2 {
		t.Fatalf("expected 3-segment token, got: %s", token)
	}

	claims, err := Parse(token, testSecret)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("UserID: got %q, want %q", claims.UserID, "user-123")
	}
	if claims.Role != "user" {
		t.Errorf("Role: got %q, want %q", claims.Role, "user")
	}
}

func TestParse_WrongSecret(t *testing.T) {
	token, err := Generate("user-123", "user", time.Hour, testSecret)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	wrongSecret := []byte("a-different-secret-32-bytes-1234")
	if _, err := Parse(token, wrongSecret); err == nil {
		t.Fatal("expected signature mismatch error")
	}
}

func TestParse_Expired(t *testing.T) {
	token, err := Generate("user-123", "user", -time.Hour, testSecret)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if _, err := Parse(token, testSecret); err == nil {
		t.Fatal("expected expired error")
	}
}

func TestParse_Malformed(t *testing.T) {
	if _, err := Parse("not.a.jwt", testSecret); err == nil {
		t.Fatal("expected parse error")
	}
	if _, err := Parse("", testSecret); err == nil {
		t.Fatal("expected parse error for empty string")
	}
}

func TestGenerate_EmptySecret(t *testing.T) {
	if _, err := Generate("user-123", "user", time.Hour, nil); err == nil {
		t.Fatal("expected error for empty secret")
	}
}

func TestParse_EmptySecret(t *testing.T) {
	if _, err := Parse("anything", nil); err == nil {
		t.Fatal("expected error for empty secret")
	}
}

func TestParse_AdminRole(t *testing.T) {
	token, err := Generate("admin-1", "admin", time.Hour, testSecret)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	claims, err := Parse(token, testSecret)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if claims.Role != "admin" {
		t.Errorf("Role: got %q, want admin", claims.Role)
	}
}

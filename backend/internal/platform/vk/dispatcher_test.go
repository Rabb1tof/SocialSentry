package vk

import (
	"errors"
	"testing"

	"go.uber.org/zap"

	"github.com/rabb1tof/socialsentry/backend/internal/domain"
)

// stubDecrypter is a TokenDecrypter test double that returns a canned value/error.
type stubDecrypter struct {
	out string
	err error
}

func (s stubDecrypter) DecryptToken(string) (string, error) { return s.out, s.err }

func TestDispatcherDecryptedAccount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		encrypted string
		decOut    string
		decErr    error
		wantToken string
		wantErr   bool
	}{
		{
			name:      "decrypts and carries plaintext on the account copy",
			encrypted: "cipher-token",
			decOut:    "plain-community-token",
			wantToken: "plain-community-token",
		},
		{
			name:      "propagates decrypt failure",
			encrypted: "cipher-token",
			decErr:    errors.New("bad key"),
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			d := NewDispatcher(nil, nil, nil, stubDecrypter{out: tc.decOut, err: tc.decErr}, nil, "5.199", zap.NewNop())
			in := domain.ConnectedAccount{
				ID:          "acc-1",
				Platform:    domain.PlatformVK,
				PlatformID:  "12345",
				AccessToken: tc.encrypted,
			}

			got, token, err := d.decryptedAccount(in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if token != tc.wantToken {
				t.Errorf("token = %q, want %q", token, tc.wantToken)
			}
			if got.AccessToken != tc.wantToken {
				t.Errorf("account.AccessToken = %q, want %q (plaintext must flow to ChooseReplyText)", got.AccessToken, tc.wantToken)
			}
			if got.ID != in.ID || got.PlatformID != in.PlatformID {
				t.Errorf("non-token fields mutated: id=%q platform_id=%q", got.ID, got.PlatformID)
			}
			if in.AccessToken != tc.encrypted {
				t.Errorf("input account was mutated in place: AccessToken = %q", in.AccessToken)
			}
		})
	}
}

func TestIsSelfSender(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		platformID string
		fromID     int
		want       bool
	}{
		{name: "own community DM (out echo)", platformID: "123456", fromID: -123456, want: true},
		{name: "own community comment", platformID: "555", fromID: -555, want: true},
		{name: "real user message", platformID: "123456", fromID: 789, want: false},
		{name: "different community", platformID: "123456", fromID: -999, want: false},
		{name: "zero from_id", platformID: "123456", fromID: 0, want: false},
		{name: "unparseable group_id", platformID: "not-a-number", fromID: -123456, want: false},
		{name: "empty group_id", platformID: "", fromID: -1, want: false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			acc := domain.ConnectedAccount{Platform: domain.PlatformVK, PlatformID: tc.platformID}
			if got := isSelfSender(acc, tc.fromID); got != tc.want {
				t.Errorf("isSelfSender(group_id=%q, from_id=%d) = %v, want %v",
					tc.platformID, tc.fromID, got, tc.want)
			}
		})
	}
}

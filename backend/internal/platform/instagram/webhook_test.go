package instagram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerifySignature_Match(t *testing.T) {
	secret := "test-app-secret"
	body := []byte(`{"object":"instagram","entry":[]}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	header := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !VerifySignature(body, header, secret) {
		t.Fatal("expected match")
	}
}

func TestVerifySignature_TamperedBody(t *testing.T) {
	secret := "test-app-secret"
	body := []byte(`{"object":"instagram","entry":[]}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	header := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	tampered := []byte(`{"object":"instagram","entry":[{"id":"evil"}]}`)
	if VerifySignature(tampered, header, secret) {
		t.Fatal("expected mismatch on tampered body")
	}
}

func TestVerifySignature_WrongSecret(t *testing.T) {
	body := []byte(`payload`)
	mac := hmac.New(sha256.New, []byte("real"))
	mac.Write(body)
	header := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if VerifySignature(body, header, "wrong") {
		t.Fatal("expected mismatch with wrong secret")
	}
}

func TestVerifySignature_MalformedHeader(t *testing.T) {
	body := []byte(`payload`)
	if VerifySignature(body, "deadbeef", "secret") {
		t.Fatal("expected false for missing prefix")
	}
	if VerifySignature(body, "sha256=zz", "secret") {
		t.Fatal("expected false for invalid hex")
	}
}

func TestParseEnvelope_DM(t *testing.T) {
	body := []byte(`{
        "object":"instagram",
        "entry":[{
            "id":"555955811455114",
            "time":1234567890,
            "messaging":[{
                "sender":{"id":"1644146630022851"},
                "recipient":{"id":"17841405879907238"},
                "timestamp":1234567890,
                "message":{"mid":"aWdf...","text":"hello there"}
            }]
        }]
    }`)
	env, err := ParseEnvelope(body)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var count int
	env.IterDMs(func(pageID string, ev MessagingEvent) {
		if pageID != "555955811455114" {
			t.Errorf("pageID: got %q want 555955811455114", pageID)
		}
		if ev.Sender.ID != "1644146630022851" {
			t.Errorf("sender: got %q", ev.Sender.ID)
		}
		if ev.Message.Text != "hello there" {
			t.Errorf("text: got %q", ev.Message.Text)
		}
		count++
	})
	if count != 1 {
		t.Errorf("got %d DMs, want 1", count)
	}
}

func TestParseEnvelope_Comment(t *testing.T) {
	body := []byte(`{
        "object":"instagram",
        "entry":[{
            "id":"555955811455114",
            "changes":[{
                "field":"comments",
                "value":{
                    "from":{"id":"1644146630022851","username":"zudina.anyaa"},
                    "media":{"id":"17841405879907238_123456","media_product_type":"FEED"},
                    "id":"17858893269000001",
                    "text":"Great post!"
                }
            }]
        }]
    }`)
	env, err := ParseEnvelope(body)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var got CommentValue
	var pageID string
	if err := env.IterComments(func(p string, c CommentValue) {
		pageID = p
		got = c
	}); err != nil {
		t.Fatalf("IterComments: %v", err)
	}
	if pageID != "555955811455114" {
		t.Errorf("pageID: got %q", pageID)
	}
	if got.From.Username != "zudina.anyaa" {
		t.Errorf("username: got %q", got.From.Username)
	}
	if got.Text != "Great post!" {
		t.Errorf("text: got %q", got.Text)
	}
}

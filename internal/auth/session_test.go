package auth

import (
	"testing"
	"time"
)

func TestSessionIssueVerify(t *testing.T) {
	manager := NewSessionManagerWithSecret([]byte("01234567890123456789012345678901"))
	token, err := manager.Issue("admin", time.Hour)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	username, err := manager.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if username != "admin" {
		t.Fatalf("Verify() username = %q, want admin", username)
	}
	if _, err := manager.Verify(token + "x"); err == nil {
		t.Fatal("Verify(tampered) error = nil, want error")
	}
}

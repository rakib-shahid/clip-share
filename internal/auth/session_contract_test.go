package auth

import (
	"strings"
	"testing"
	"time"
)

func TestSessionContract(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	manager := NewSessionManager([]byte("01234567890123456789012345678901"))
	manager.clock = func() time.Time { return now }
	token := manager.Create(42)
	if id, err := manager.Parse(token); err != nil || id != 42 {
		t.Fatalf("parse=%d err=%v", id, err)
	}
	if _, err := manager.Parse(token + "x"); err == nil {
		t.Fatal("tampered token accepted")
	}
	manager.clock = func() time.Time { return now.Add(SessionLifetime + time.Second) }
	if _, err := manager.Parse(token); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expiry err=%v", err)
	}
}

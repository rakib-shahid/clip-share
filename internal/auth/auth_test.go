package auth

import (
	"strings"
	"testing"
	"time"
)

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hash, "correct horse") {
		t.Fatal("hash contains password")
	}
	ok, err := VerifyPassword("correct horse battery staple", hash)
	if err != nil || !ok {
		t.Fatalf("verify correct password: ok=%v err=%v", ok, err)
	}
	ok, err = VerifyPassword("wrong password", hash)
	if err != nil || ok {
		t.Fatalf("verify wrong password: ok=%v err=%v", ok, err)
	}
}

func TestSessionRoundTripAndExpiry(t *testing.T) {
	manager := NewSessionManager([]byte("01234567890123456789012345678901"))
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	manager.clock = func() time.Time { return now }
	token := manager.Create(42)
	userID, err := manager.Parse(token)
	if err != nil || userID != 42 {
		t.Fatalf("parse: userID=%d err=%v", userID, err)
	}
	if _, err := manager.Parse(token + "tampered"); err == nil {
		t.Fatal("tampered token was accepted")
	}
	manager.clock = func() time.Time { return now.Add(SessionLifetime + time.Second) }
	if _, err := manager.Parse(token); err == nil {
		t.Fatal("expired token was accepted")
	}
}

func TestUsernameValidation(t *testing.T) {
	tests := []struct {
		input string
		want  string
		valid bool
	}{
		{"Alice", "alice", true},
		{" user-name ", "user-name", true},
		{"no spaces", "", false},
		{"ab", "", false},
	}
	for _, test := range tests {
		got, err := NormalizeUsername(test.input)
		if test.valid && (err != nil || got != test.want) {
			t.Errorf("%q: got %q, %v", test.input, got, err)
		}
		if !test.valid && err == nil {
			t.Errorf("%q: expected error", test.input)
		}
	}
}

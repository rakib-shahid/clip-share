package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const SessionLifetime = 24 * time.Hour

type SessionManager struct {
	secret []byte
	clock  func() time.Time
}

func NewSessionManager(secret []byte) *SessionManager {
	return &SessionManager{secret: secret, clock: time.Now}
}

func (m *SessionManager) Create(userID int64) string {
	payload := fmt.Sprintf("%d:%d", userID, m.clock().UTC().Unix())
	return encode(payload) + "." + encodeBytes(m.sign("session:"+payload))
}

func (m *SessionManager) Parse(token string) (int64, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return 0, errors.New("invalid session")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, errors.New("invalid session")
	}
	payload := string(payloadBytes)
	want := encodeBytes(m.sign("session:" + payload))
	if !hmac.Equal([]byte(parts[1]), []byte(want)) {
		return 0, errors.New("invalid session")
	}
	fields := strings.Split(payload, ":")
	if len(fields) != 2 {
		return 0, errors.New("invalid session")
	}
	userID, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil || userID <= 0 {
		return 0, errors.New("invalid session")
	}
	issuedUnix, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return 0, errors.New("invalid session")
	}
	issued := time.Unix(issuedUnix, 0)
	now := m.clock()
	if issued.After(now.Add(time.Minute)) || now.Sub(issued) > SessionLifetime {
		return 0, errors.New("expired session")
	}
	return userID, nil
}

func (m *SessionManager) CSRF(sessionToken string) string {
	return encodeBytes(m.sign("csrf:" + sessionToken))
}

func (m *SessionManager) VerifyCSRF(sessionToken, csrfToken string) bool {
	return hmac.Equal([]byte(m.CSRF(sessionToken)), []byte(csrfToken))
}

func (m *SessionManager) sign(value string) []byte {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(value))
	return mac.Sum(nil)
}

func encode(value string) string      { return base64.RawURLEncoding.EncodeToString([]byte(value)) }
func encodeBytes(value []byte) string { return base64.RawURLEncoding.EncodeToString(value) }

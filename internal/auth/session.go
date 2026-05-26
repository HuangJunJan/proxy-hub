package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const SessionCookieName = "proxy_hub_session"

type SessionManager struct {
	secret []byte
	now    func() time.Time
}

func NewSessionManager() (*SessionManager, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate session secret: %w", err)
	}
	return &SessionManager{secret: secret, now: time.Now}, nil
}

func NewSessionManagerWithSecret(secret []byte) *SessionManager {
	return &SessionManager{secret: append([]byte(nil), secret...), now: time.Now}
}

func (m *SessionManager) Issue(username string, ttl time.Duration) (string, error) {
	if username == "" {
		return "", errors.New("username is required")
	}
	if len(m.secret) == 0 {
		return "", errors.New("session secret is empty")
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate session nonce: %w", err)
	}
	expires := m.now().Add(ttl).Unix()
	enc := base64.RawURLEncoding
	payload := strings.Join([]string{
		enc.EncodeToString([]byte(username)),
		strconv.FormatInt(expires, 10),
		enc.EncodeToString(nonce),
	}, ".")
	sig := sign(m.secret, payload)
	return payload + "." + enc.EncodeToString(sig), nil
}

func (m *SessionManager) Verify(token string) (string, error) {
	if len(m.secret) == 0 {
		return "", errors.New("session secret is empty")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 4 {
		return "", errors.New("invalid session token")
	}
	payload := strings.Join(parts[:3], ".")
	expectedSig := sign(m.secret, payload)
	actualSig, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return "", errors.New("invalid session signature")
	}
	if !hmac.Equal(actualSig, expectedSig) {
		return "", errors.New("invalid session signature")
	}
	expires, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", errors.New("invalid session expiry")
	}
	if m.now().Unix() > expires {
		return "", errors.New("session expired")
	}
	usernameBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", errors.New("invalid session username")
	}
	username := string(usernameBytes)
	if username == "" {
		return "", errors.New("invalid session username")
	}
	return username, nil
}

func (m *SessionManager) SetCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int((30 * 24 * time.Hour).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func sign(secret []byte, payload string) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}

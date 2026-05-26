package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func GenerateProxyAPIKey() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate api key: %w", err)
	}
	return "sk-proxy-hub-" + base64.RawURLEncoding.EncodeToString(raw), nil
}

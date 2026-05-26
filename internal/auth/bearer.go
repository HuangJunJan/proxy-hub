package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"proxy-hub/internal/config"
)

type APIKeyPrincipal struct {
	Token string
	Name  string
}

func AuthenticateBearer(r *http.Request, cfg *config.Config) (APIKeyPrincipal, bool) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return APIKeyPrincipal{}, false
	}
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return APIKeyPrincipal{}, false
	}
	for _, key := range cfg.APIKeys {
		if key.Disabled {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(key.Token)) == 1 {
			return APIKeyPrincipal{Token: key.Token, Name: key.Name}, true
		}
	}
	return APIKeyPrincipal{}, false
}

func MaskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

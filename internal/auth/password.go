package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    uint32 = 3
	argonMemory  uint32 = 64 * 1024
	argonThreads uint8  = 2
	argonKeyLen  uint32 = 32
	saltLen             = 16
)

func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("password is required")
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	enc := base64.RawStdEncoding
	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory,
		argonTime,
		argonThreads,
		enc.EncodeToString(salt),
		enc.EncodeToString(hash),
	), nil
}

func VerifyPassword(encodedHash, password string) (bool, error) {
	params, salt, expected, err := parseArgon2ID(encodedHash)
	if err != nil {
		return false, err
	}
	actual := argon2.IDKey([]byte(password), salt, params.time, params.memory, params.threads, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func IsArgon2IDHash(value string) bool {
	return strings.HasPrefix(value, "$argon2id$")
}

type argonParams struct {
	memory  uint32
	time    uint32
	threads uint8
}

func parseArgon2ID(encodedHash string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return argonParams{}, nil, nil, errors.New("invalid argon2id hash format")
	}
	var params argonParams
	for _, part := range strings.Split(parts[3], ",") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			return argonParams{}, nil, nil, errors.New("invalid argon2id parameter")
		}
		n, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return argonParams{}, nil, nil, fmt.Errorf("parse argon2id parameter %q: %w", key, err)
		}
		switch key {
		case "m":
			params.memory = uint32(n)
		case "t":
			params.time = uint32(n)
		case "p":
			params.threads = uint8(n)
		default:
			return argonParams{}, nil, nil, fmt.Errorf("unknown argon2id parameter %q", key)
		}
	}
	enc := base64.RawStdEncoding
	salt, err := enc.DecodeString(parts[4])
	if err != nil {
		return argonParams{}, nil, nil, fmt.Errorf("decode argon2id salt: %w", err)
	}
	hash, err := enc.DecodeString(parts[5])
	if err != nil {
		return argonParams{}, nil, nil, fmt.Errorf("decode argon2id hash: %w", err)
	}
	if params.memory == 0 || params.time == 0 || params.threads == 0 || len(salt) == 0 || len(hash) == 0 {
		return argonParams{}, nil, nil, errors.New("invalid argon2id hash values")
	}
	return params, salt, hash, nil
}

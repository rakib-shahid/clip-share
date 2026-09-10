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
	argonMemory      = 19 * 1024
	argonIterations  = 2
	argonParallelism = 1
	argonSaltLength  = 16
	argonKeyLength   = 32
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLength)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonIterations, argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash)), nil
}

func VerifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false, errors.New("invalid password hash format")
	}
	var memory uint64
	var iterations uint64
	var parallelism uint64
	for _, pair := range strings.Split(parts[3], ",") {
		fields := strings.SplitN(pair, "=", 2)
		if len(fields) != 2 {
			return false, errors.New("invalid password hash parameters")
		}
		value, err := strconv.ParseUint(fields[1], 10, 32)
		if err != nil {
			return false, errors.New("invalid password hash parameters")
		}
		switch fields[0] {
		case "m":
			memory = value
		case "t":
			iterations = value
		case "p":
			parallelism = value
		}
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, errors.New("invalid password hash salt")
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, errors.New("invalid password hash value")
	}
	if memory == 0 || iterations == 0 || parallelism == 0 || len(want) == 0 {
		return false, errors.New("invalid password hash parameters")
	}
	got := argon2.IDKey([]byte(password), salt, uint32(iterations), uint32(memory), uint8(parallelism), uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

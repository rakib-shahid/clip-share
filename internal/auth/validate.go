package auth

import (
	"errors"
	"regexp"
	"strings"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{3,32}$`)

func NormalizeUsername(username string) (string, error) {
	username = strings.TrimSpace(username)
	if !usernamePattern.MatchString(username) {
		return "", errors.New("username must be 3-32 letters, numbers, underscores, or hyphens")
	}
	return strings.ToLower(username), nil
}

func ValidatePassword(password string) error {
	length := len([]rune(password))
	if length < 8 || length > 128 {
		return errors.New("password must be 8-128 characters")
	}
	return nil
}

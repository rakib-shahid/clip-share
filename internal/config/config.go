package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Address       string
	BaseURL       string
	DataDir       string
	DatabasePath  string
	SessionSecret []byte
	SetupToken    string
	SecureCookies bool
	AllowedOrigin string
}

func Load() (Config, error) {
	dataDir := envOr("CLIP_SHARE_DATA_DIR", "/data")
	secureCookies, err := strconv.ParseBool(envOr("CLIP_SHARE_SECURE_COOKIES", "true"))
	if err != nil {
		return Config{}, fmt.Errorf("CLIP_SHARE_SECURE_COOKIES: %w", err)
	}

	secretText := os.Getenv("CLIP_SHARE_SESSION_SECRET")
	if secretText == "" {
		return Config{}, errors.New("CLIP_SHARE_SESSION_SECRET is required")
	}
	secret, err := decodeSecret(secretText)
	if err != nil {
		return Config{}, fmt.Errorf("CLIP_SHARE_SESSION_SECRET: %w", err)
	}
	if len(secret) < 32 {
		return Config{}, errors.New("CLIP_SHARE_SESSION_SECRET must contain at least 32 bytes")
	}

	setupToken := os.Getenv("CLIP_SHARE_SETUP_TOKEN")
	if len(setupToken) < 32 {
		return Config{}, errors.New("CLIP_SHARE_SETUP_TOKEN must be at least 32 characters")
	}

	return Config{
		Address:       envOr("CLIP_SHARE_ADDR", ":8080"),
		BaseURL:       envOr("CLIP_SHARE_BASE_URL", "http://localhost:8080"),
		DataDir:       dataDir,
		DatabasePath:  filepath.Join(dataDir, "database", "clip-share.db"),
		SessionSecret: secret,
		SetupToken:    setupToken,
		SecureCookies: secureCookies,
		// The fixed TrueNAS LAN address is also the safe default for direct
		// access when a deployment omits the optional environment variable.
		AllowedOrigin: envOr("CLIP_SHARE_ALLOWED_ORIGIN", "http://192.168.1.66:43138"),
	}, nil
}

func PrepareDataDirectories(cfg Config) error {
	for _, directory := range []string{
		filepath.Join(cfg.DataDir, "database"),
		filepath.Join(cfg.DataDir, "media"),
		filepath.Join(cfg.DataDir, "temporary"),
	} {
		if err := os.MkdirAll(directory, 0o750); err != nil {
			return fmt.Errorf("prepare data directory: %w", err)
		}
	}
	return nil
}

func decodeSecret(value string) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err == nil {
		return decoded, nil
	}
	return []byte(value), nil
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

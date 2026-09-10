package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

var ErrAlreadySetup = errors.New("application is already set up")
var ErrUsernameTaken = errors.New("username is already in use")

type User struct {
	ID                   int64  `json:"id"`
	Username             string `json:"username"`
	Role                 string `json:"role"`
	State                string `json:"state"`
	StoredFileLimitBytes int64  `json:"storedFileLimitBytes"`
	RootFolderID         int64  `json:"rootFolderId"`
	LibraryTrashed       bool   `json:"libraryTrashed"`
}

// UserStorageSummary is returned only by administrator user-list operations.
// It deliberately does not change the User shape used by sessions and normal
// user-facing endpoints.
type UserStorageSummary struct {
	User
	StoredBytes int64 `json:"storedBytes"`
}

type Store struct {
	db *sql.DB
}

func Open(databasePath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o750); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	for _, pragma := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("configure sqlite: %w", err)
		}
	}

	store := &Store{db: db}
	if err := store.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Ready(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) IsSetup(ctx context.Context) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count)
	return count == 1, err
}

func (s *Store) CreateAdmin(ctx context.Context, username, normalized, passwordHash string, limit int64) (User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()

	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count); err != nil {
		return User{}, err
	}
	if count != 0 {
		return User{}, ErrAlreadySetup
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `
		INSERT INTO users(username, username_normalized, password_hash, role, state, stored_file_limit_bytes, created_at, updated_at)
		VALUES (?, ?, ?, 'admin', 'active', ?, ?, ?)`, username, normalized, passwordHash, limit, now, now)
	if err != nil {
		return User{}, classifyConstraint(err)
	}
	userID, err := result.LastInsertId()
	if err != nil {
		return User{}, err
	}
	rootResult, err := tx.ExecContext(ctx, `
		INSERT INTO folders(owner_user_id, name, name_normalized, is_root, created_at, updated_at)
		VALUES (?, ?, ?, 1, ?, ?)`, userID, username, normalized, now, now)
	if err != nil {
		return User{}, err
	}
	rootID, err := rootResult.LastInsertId()
	if err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return User{ID: userID, Username: username, Role: "admin", State: "active", StoredFileLimitBytes: limit, RootFolderID: rootID}, nil
}

func (s *Store) CreateUser(ctx context.Context, username, normalized, passwordHash string, limit int64) (User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `
		INSERT INTO users(username, username_normalized, password_hash, role, state, stored_file_limit_bytes, created_at, updated_at)
		VALUES (?, ?, ?, 'user', 'active', ?, ?, ?)`, username, normalized, passwordHash, limit, now, now)
	if err != nil {
		return User{}, classifyConstraint(err)
	}
	userID, err := result.LastInsertId()
	if err != nil {
		return User{}, err
	}
	rootResult, err := tx.ExecContext(ctx, `
		INSERT INTO folders(owner_user_id, name, name_normalized, is_root, created_at, updated_at)
		VALUES (?, ?, ?, 1, ?, ?)`, userID, username, normalized, now, now)
	if err != nil {
		return User{}, err
	}
	rootID, err := rootResult.LastInsertId()
	if err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return User{ID: userID, Username: username, Role: "user", State: "active", StoredFileLimitBytes: limit, RootFolderID: rootID}, nil
}

func (s *Store) UserForLogin(ctx context.Context, normalized string) (User, string, error) {
	var user User
	var hash string
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.role, u.state, u.stored_file_limit_bytes, f.id, f.deleted_at IS NOT NULL, u.password_hash
		FROM users u JOIN folders f ON f.owner_user_id = u.id AND f.is_root = 1
		WHERE u.username_normalized = ?`, normalized).
		Scan(&user.ID, &user.Username, &user.Role, &user.State, &user.StoredFileLimitBytes, &user.RootFolderID, &user.LibraryTrashed, &hash)
	return user, hash, err
}

func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.role, u.state, u.stored_file_limit_bytes, f.id, f.deleted_at IS NOT NULL
		FROM users u JOIN folders f ON f.owner_user_id = u.id AND f.is_root = 1
		WHERE u.id = ?`, id).
		Scan(&user.ID, &user.Username, &user.Role, &user.State, &user.StoredFileLimitBytes, &user.RootFolderID, &user.LibraryTrashed)
	return user, err
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.username, u.role, u.state, u.stored_file_limit_bytes, f.id, f.deleted_at IS NOT NULL
		FROM users u JOIN folders f ON f.owner_user_id = u.id AND f.is_root = 1
		ORDER BY CASE u.role WHEN 'admin' THEN 0 ELSE 1 END, u.username_normalized`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]User, 0)
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username, &user.Role, &user.State, &user.StoredFileLimitBytes, &user.RootFolderID, &user.LibraryTrashed); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *Store) ListUserStorageSummaries(ctx context.Context) ([]UserStorageSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.username, u.role, u.state, u.stored_file_limit_bytes, f.id,
		       f.deleted_at IS NOT NULL, COALESCE(usage.stored_bytes, 0)
		FROM users u
		JOIN folders f ON f.owner_user_id = u.id AND f.is_root = 1
		LEFT JOIN (
			SELECT owner_user_id, SUM(size_bytes) AS stored_bytes
			FROM clips WHERE state = 'ready' GROUP BY owner_user_id
		) usage ON usage.owner_user_id = u.id
		ORDER BY CASE u.role WHEN 'admin' THEN 0 ELSE 1 END, u.username_normalized`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]UserStorageSummary, 0)
	for rows.Next() {
		var user UserStorageSummary
		if err := rows.Scan(&user.ID, &user.Username, &user.Role, &user.State, &user.StoredFileLimitBytes, &user.RootFolderID, &user.LibraryTrashed, &user.StoredBytes); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}
	entries, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(entries)
	for _, name := range entries {
		base := filepath.Base(name)
		versionText := strings.SplitN(base, "_", 2)[0]
		version, err := strconv.Atoi(versionText)
		if err != nil {
			return fmt.Errorf("invalid migration filename %q", name)
		}
		var exists int
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&exists); err != nil {
			return err
		}
		if exists != 0 {
			continue
		}
		script, err := migrationFiles.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(script)); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", version, err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)", version, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func classifyConstraint(err error) error {
	if strings.Contains(strings.ToLower(err.Error()), "unique constraint") {
		return ErrUsernameTaken
	}
	return err
}

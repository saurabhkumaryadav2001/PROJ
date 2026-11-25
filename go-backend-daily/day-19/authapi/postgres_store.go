package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type PostgresStore struct{ db *sql.DB }

func NewPostgresStore(db *sql.DB) *PostgresStore { return &PostgresStore{db: db} }

func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *PostgresStore) CreateUser(ctx context.Context, user User) error {
	const query = `INSERT INTO app_users (id, email, password_hash, role) VALUES ($1, $2, $3, $4)`
	_, err := s.db.ExecContext(ctx, query, user.ID, user.Email, user.PasswordHash, string(user.Role))
	if isUniqueViolation(err) {
		return ErrEmailExists
	}
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

func (s *PostgresStore) UserByEmail(ctx context.Context, email string) (User, error) {
	const query = `SELECT id::text, email, password_hash, role FROM app_users WHERE email = $1 AND disabled_at IS NULL`
	var user User
	var role string
	err := s.db.QueryRowContext(ctx, query, email).Scan(&user.ID, &user.Email, &user.PasswordHash, &role)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("select user by email: %w", err)
	}
	user.Role = Role(role)
	return user, nil
}

func (s *PostgresStore) UserByID(ctx context.Context, id string) (User, error) {
	const query = `SELECT id::text, email, password_hash, role FROM app_users WHERE id = $1 AND disabled_at IS NULL`
	var user User
	var role string
	err := s.db.QueryRowContext(ctx, query, id).Scan(&user.ID, &user.Email, &user.PasswordHash, &role)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("select user by ID: %w", err)
	}
	user.Role = Role(role)
	return user, nil
}

func (s *PostgresStore) CreateRefresh(ctx context.Context, session RefreshSession) error {
	const query = `INSERT INTO refresh_sessions (id, family_id, token_hash, user_id, expires_at) VALUES ($1, $2, $3, $4, $5)`
	_, err := s.db.ExecContext(ctx, query, session.ID, session.FamilyID, session.TokenHash, session.UserID, session.ExpiresAt)
	if err != nil {
		return fmt.Errorf("insert refresh session: %w", err)
	}
	return nil
}

// RotateRefresh locks the old token row, inserts its replacement, and marks the
// old token consumed in ONE transaction. Reuse revokes the entire token family.
func (s *PostgresStore) RotateRefresh(ctx context.Context, oldHash string, replacement RefreshSession, now time.Time) (User, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return User{}, fmt.Errorf("begin refresh rotation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const selectOld = `
		SELECT rs.id::text, rs.family_id::text, rs.user_id::text,
		       rs.expires_at, rs.consumed_at, rs.revoked_at,
		       u.email, u.password_hash, u.role, u.disabled_at
		FROM refresh_sessions rs
		JOIN app_users u ON u.id = rs.user_id
		WHERE rs.token_hash = $1
		FOR UPDATE OF rs`
	var oldID, familyID, userID, role string
	var expiresAt time.Time
	var consumedAt, revokedAt, disabledAt sql.NullTime
	var user User
	err = tx.QueryRowContext(ctx, selectOld, oldHash).Scan(
		&oldID, &familyID, &userID, &expiresAt, &consumedAt, &revokedAt,
		&user.Email, &user.PasswordHash, &role, &disabledAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrRefreshInvalid
	}
	if err != nil {
		return User{}, fmt.Errorf("lock refresh session: %w", err)
	}

	if consumedAt.Valid || revokedAt.Valid {
		if _, err := tx.ExecContext(ctx,
			`UPDATE refresh_sessions SET revoked_at = COALESCE(revoked_at, $2) WHERE family_id = $1`, familyID, now,
		); err != nil {
			return User{}, fmt.Errorf("revoke reused token family: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return User{}, fmt.Errorf("commit reuse revocation: %w", err)
		}
		return User{}, ErrRefreshReuse
	}
	if !expiresAt.After(now) || disabledAt.Valid {
		if _, err := tx.ExecContext(ctx, `UPDATE refresh_sessions SET revoked_at = $2 WHERE family_id = $1 AND revoked_at IS NULL`, familyID, now); err != nil {
			return User{}, fmt.Errorf("revoke expired or disabled session family: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return User{}, fmt.Errorf("commit expired session revocation: %w", err)
		}
		return User{}, ErrRefreshInvalid
	}

	const insertReplacement = `
		INSERT INTO refresh_sessions (id, family_id, token_hash, user_id, expires_at)
		VALUES ($1, $2, $3, $4, $5)`
	if _, err := tx.ExecContext(ctx, insertReplacement,
		replacement.ID, familyID, replacement.TokenHash, userID, replacement.ExpiresAt,
	); err != nil {
		return User{}, fmt.Errorf("insert replacement refresh session: %w", err)
	}

	result, err := tx.ExecContext(ctx,
		`UPDATE refresh_sessions SET consumed_at = $1, replaced_by_id = $2 WHERE id = $3 AND consumed_at IS NULL AND revoked_at IS NULL`,
		now, replacement.ID, oldID,
	)
	if err != nil {
		return User{}, fmt.Errorf("consume old refresh session: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return User{}, fmt.Errorf("check refresh rotation result: %w", err)
	}
	if rows != 1 {
		return User{}, ErrRefreshInvalid
	}
	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("commit refresh rotation: %w", err)
	}

	user.ID = userID
	user.Role = Role(role)
	return user, nil
}

func (s *PostgresStore) RevokeRefresh(ctx context.Context, tokenHash string, now time.Time) error {
	const query = `
		UPDATE refresh_sessions
		SET revoked_at = $2
		WHERE family_id = (SELECT family_id FROM refresh_sessions WHERE token_hash = $1)
		  AND revoked_at IS NULL`
	result, err := s.db.ExecContext(ctx, query, tokenHash, now)
	if err != nil {
		return fmt.Errorf("revoke refresh token family: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check revoked sessions: %w", err)
	}
	if rows == 0 {
		return ErrRefreshInvalid
	}
	return nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var state interface{ SQLState() string }
	return errors.As(err, &state) && state.SQLState() == "23505"
}

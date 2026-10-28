package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/Xumeiquer/asesoria/internal/domain"
)

const pairingTTL = 10 * time.Minute

func (s *Store) CreatePairingCode(ctx context.Context) (domain.PairingCode, error) {
	now := time.Now().UTC()
	pc := domain.PairingCode{
		CreatedAt: now.Format(time.RFC3339),
		ExpiresAt: now.Add(pairingTTL).Format(time.RFC3339),
	}
	_, _ = s.DB.ExecContext(ctx, `DELETE FROM pairing_codes WHERE expires_at < ? OR used_at IS NOT NULL`, now.Format(time.RFC3339))

	for attempt := 0; attempt < 10; attempt++ {
		n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
		if err != nil {
			return pc, err
		}
		pc.Code = fmt.Sprintf("%06d", n.Int64())
		_, err = s.DB.ExecContext(ctx, `INSERT INTO pairing_codes (code, created_at, expires_at) VALUES (?, ?, ?)`,
			pc.Code, pc.CreatedAt, pc.ExpiresAt)
		if err == nil {
			return pc, nil
		}
	}
	return pc, errors.New("could not generate a pairing code")
}

// ClaimPairingCode exchanges a valid code for a long-lived extension token.
func (s *Store) ClaimPairingCode(ctx context.Context, code, name string) (string, domain.APIToken, error) {
	var token domain.APIToken
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", token, err
	}
	defer tx.Rollback()

	var expiresAt string
	var usedAt sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT expires_at, used_at FROM pairing_codes WHERE code = ?`, code).Scan(&expiresAt, &usedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", token, ErrNotFound
	}
	if err != nil {
		return "", token, err
	}
	if usedAt.Valid {
		return "", token, errors.New("pairing code already used")
	}
	exp, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil || time.Now().UTC().After(exp) {
		return "", token, errors.New("pairing code expired")
	}

	token.ID = NewID()
	if name == "" {
		name = "Extensión"
	}
	token.Name = name
	token.CreatedAt = Now()
	raw := randomHex(32)
	hash := hashToken(raw)

	if _, err := tx.ExecContext(ctx, `UPDATE pairing_codes SET used_at = ? WHERE code = ?`, token.CreatedAt, code); err != nil {
		return "", token, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO api_tokens (id, name, token_hash, created_at) VALUES (?, ?, ?, ?)`,
		token.ID, token.Name, hash, token.CreatedAt); err != nil {
		return "", token, err
	}
	if err := tx.Commit(); err != nil {
		return "", token, err
	}
	return raw, token, nil
}

func (s *Store) ValidateToken(ctx context.Context, raw string) (domain.APIToken, bool) {
	if raw == "" {
		return domain.APIToken{}, false
	}
	var token domain.APIToken
	var lastUsed sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT id, name, created_at, last_used_at FROM api_tokens WHERE token_hash = ?`,
		hashToken(raw)).Scan(&token.ID, &token.Name, &token.CreatedAt, &lastUsed)
	if err != nil {
		return domain.APIToken{}, false
	}
	if lastUsed.Valid {
		token.LastUsedAt = &lastUsed.String
	}
	// Avoid a write per request: update at most once a minute.
	if !lastUsed.Valid || time.Since(mustParse(lastUsed.String)) > time.Minute {
		now := Now()
		_, _ = s.DB.ExecContext(ctx, `UPDATE api_tokens SET last_used_at = ? WHERE id = ?`, now, token.ID)
		token.LastUsedAt = &now
	}
	return token, true
}

func (s *Store) ListTokens(ctx context.Context) ([]domain.APIToken, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, created_at, last_used_at FROM api_tokens ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.APIToken, 0)
	for rows.Next() {
		var t domain.APIToken
		var lastUsed sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &lastUsed); err != nil {
			return nil, err
		}
		if lastUsed.Valid {
			t.LastUsedAt = &lastUsed.String
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) RevokeToken(ctx context.Context, id string) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM api_tokens WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func mustParse(value string) time.Time {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return t
}

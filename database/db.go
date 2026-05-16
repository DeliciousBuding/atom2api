package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func Open(dbPath string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	sqldb, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(15000)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	sqldb.SetMaxOpenConns(1)
	db := &DB{sqldb}
	if err := db.migrate(); err != nil {
		sqldb.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func (db *DB) migrate() error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS tokens (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    label           TEXT DEFAULT '',
    access_token    TEXT NOT NULL,
    refresh_token   TEXT DEFAULT '',
    user_info       TEXT DEFAULT '{}',
    status          TEXT DEFAULT 'active',
    last_used_at    TIMESTAMP NULL,
    last_refreshed_at TIMESTAMP NULL,
    error_message   TEXT DEFAULT '',
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS api_keys (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT DEFAULT '',
    key             TEXT NOT NULL UNIQUE,
    enabled         INTEGER DEFAULT 1,
    quota_limit     INTEGER DEFAULT 0,
    quota_used      INTEGER DEFAULT 0,
    expires_at      TIMESTAMP NULL,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS usage_logs (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    token_id         INTEGER DEFAULT 0,
    api_key_id       INTEGER DEFAULT 0,
    model            TEXT DEFAULT '',
    endpoint         TEXT DEFAULT '',
    is_stream        INTEGER DEFAULT 0,
    prompt_tokens    INTEGER DEFAULT 0,
    completion_tokens INTEGER DEFAULT 0,
    total_tokens     INTEGER DEFAULT 0,
    status_code      INTEGER DEFAULT 0,
    duration_ms      INTEGER DEFAULT 0,
    error_message    TEXT DEFAULT '',
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);

INSERT OR IGNORE INTO settings (key, value) VALUES
    ('admin_secret', ''),
    ('default_model', 'deepseek-v4-flash'),
    ('rate_limit_rpm', '0'),
    ('auto_refresh_tokens', 'true');
`)
	return err
}

// --- Token CRUD ---

func (db *DB) ListTokens() ([]Token, error) {
	rows, err := db.Query(`SELECT id,label,access_token,refresh_token,user_info,status,last_used_at,last_refreshed_at,error_message,created_at,updated_at FROM tokens ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Token
	for rows.Next() {
		t, err := scanToken(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (db *DB) GetActiveTokens() ([]Token, error) {
	rows, err := db.Query(`SELECT id,label,access_token,refresh_token,user_info,status,last_used_at,last_refreshed_at,error_message,created_at,updated_at FROM tokens WHERE status='active' ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Token
	for rows.Next() {
		t, err := scanToken(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (db *DB) InsertToken(label, accessToken, refreshToken, userInfo string) (int64, error) {
	res, err := db.Exec(`INSERT INTO tokens (label, access_token, refresh_token, user_info) VALUES (?,?,?,?)`,
		label, accessToken, refreshToken, userInfo)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) UpdateTokenStatus(id int64, status, errMsg string) error {
	_, err := db.Exec(`UPDATE tokens SET status=?, error_message=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, status, errMsg, id)
	return err
}

func (db *DB) UpdateTokenTokens(id int64, accessToken, refreshToken string) error {
	_, err := db.Exec(`UPDATE tokens SET access_token=?, refresh_token=?, status='active', error_message='', last_refreshed_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=?`, accessToken, refreshToken, id)
	return err
}

func (db *DB) TouchToken(id int64) error {
	_, err := db.Exec(`UPDATE tokens SET last_used_at=CURRENT_TIMESTAMP WHERE id=?`, id)
	return err
}

func (db *DB) DeleteToken(id int64) error {
	_, err := db.Exec(`DELETE FROM tokens WHERE id=?`, id)
	return err
}

func (db *DB) TokenExists(accessToken string) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM tokens WHERE access_token=?`, accessToken).Scan(&count)
	return count > 0, err
}

func (db *DB) TokenCount() (total, active, errCount int, err error) {
	err = db.QueryRow(`SELECT COUNT(*) FROM tokens`).Scan(&total)
	if err != nil {
		return
	}
	err = db.QueryRow(`SELECT COUNT(*) FROM tokens WHERE status='active'`).Scan(&active)
	if err != nil {
		return
	}
	err = db.QueryRow(`SELECT COUNT(*) FROM tokens WHERE status='error'`).Scan(&errCount)
	return
}

// --- API Key CRUD ---

func (db *DB) ListAPIKeys() ([]APIKey, error) {
	rows, err := db.Query(`SELECT id,name,key,enabled,quota_limit,quota_used,expires_at,created_at FROM api_keys ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APIKey
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *k)
	}
	return out, rows.Err()
}

func (db *DB) HasAPIKeys() (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM api_keys WHERE enabled=1`).Scan(&count)
	return count > 0, err
}

func (db *DB) ValidateAPIKey(key string) (*APIKey, error) {
	k, err := scanAPIKey(db.QueryRow(`SELECT id,name,key,enabled,quota_limit,quota_used,expires_at,created_at FROM api_keys WHERE key=? AND enabled=1`, key))
	if err != nil {
		return nil, err
	}
	if k.ExpiresAt != nil && k.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("api key expired")
	}
	if k.QuotaLimit > 0 && k.QuotaUsed >= k.QuotaLimit {
		return nil, fmt.Errorf("api key quota exceeded")
	}
	return k, nil
}

func (db *DB) InsertAPIKey(name, key string, quotaLimit int64, expiresAt *time.Time) (int64, error) {
	res, err := db.Exec(`INSERT INTO api_keys (name, key, quota_limit, expires_at) VALUES (?,?,?,?)`, name, key, quotaLimit, expiresAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) DeleteAPIKey(id int64) error {
	_, err := db.Exec(`DELETE FROM api_keys WHERE id=?`, id)
	return err
}

func (db *DB) IncrementAPIKeyUsage(id int64) error {
	_, err := db.Exec(`UPDATE api_keys SET quota_used=quota_used+1 WHERE id=?`, id)
	return err
}

func (db *DB) InsertUsageLog(l *UsageLog) error {
	_, err := db.Exec(`INSERT INTO usage_logs (token_id,api_key_id,model,endpoint,is_stream,prompt_tokens,completion_tokens,total_tokens,status_code,duration_ms,error_message) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		l.TokenID, l.APIKeyID, l.Model, l.Endpoint, boolToInt(l.IsStream),
		l.PromptTokens, l.CompletionTokens, l.TotalTokens, l.StatusCode, l.DurationMs, l.ErrorMessage)
	return err
}

func (db *DB) ListUsageLogs(limit, offset int, model string) ([]UsageLog, int, error) {
	where := "1=1"
	args := []any{}
	if model != "" {
		where += " AND model=?"
		args = append(args, model)
	}
	var total int
	err := db.QueryRow("SELECT COUNT(*) FROM usage_logs WHERE "+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}
	queryArgs := append(args, limit, offset)
	rows, err := db.Query("SELECT id,token_id,api_key_id,model,endpoint,is_stream,prompt_tokens,completion_tokens,total_tokens,status_code,duration_ms,error_message,created_at FROM usage_logs WHERE "+where+" ORDER BY id DESC LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []UsageLog
	for rows.Next() {
		l, err := scanUsageLog(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *l)
	}
	return out, total, rows.Err()
}

func (db *DB) UsageStats() (totalReqs int, totalTokens int, err error) {
	err = db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(total_tokens),0) FROM usage_logs`).Scan(&totalReqs, &totalTokens)
	return
}

// --- Settings ---

func (db *DB) GetSetting(key string) (string, error) {
	var val string
	err := db.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

func (db *DB) SetSetting(key, value string) error {
	_, err := db.Exec(`INSERT INTO settings (key, value) VALUES (?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func (db *DB) AllSettings() (map[string]string, error) {
	rows, err := db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		m[k] = v
	}
	return m, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

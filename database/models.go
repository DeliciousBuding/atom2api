package database

import (
	"database/sql"
	"time"
)

type Token struct {
	ID              int64      `json:"id"`
	Label           string     `json:"label"`
	AccessToken     string     `json:"access_token"`
	RefreshToken    string     `json:"refresh_token,omitempty"`
	UserInfo        string     `json:"user_info"`
	Status          string     `json:"status"`
	Enabled         bool       `json:"enabled"`
	QuotaInfo       string     `json:"quota_info"`
	LastUsedAt      *time.Time `json:"last_used_at"`
	LastRefreshedAt *time.Time `json:"last_refreshed_at"`
	LastTestedAt    *time.Time `json:"last_tested_at"`
	LastQuotaAt     *time.Time `json:"last_quota_at"`
	TestLatencyMs   int        `json:"test_latency_ms"`
	ErrorMessage    string     `json:"error_message"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type APIKey struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Key        string     `json:"key"`
	Enabled    bool       `json:"enabled"`
	QuotaLimit int64      `json:"quota_limit"`
	QuotaUsed  int64      `json:"quota_used"`
	ExpiresAt  *time.Time `json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

type UsageLog struct {
	ID               int64     `json:"id"`
	TokenID          int64     `json:"token_id"`
	APIKeyID         int64     `json:"api_key_id"`
	Model            string    `json:"model"`
	Endpoint         string    `json:"endpoint"`
	IsStream         bool      `json:"is_stream"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	StatusCode       int       `json:"status_code"`
	DurationMs       int       `json:"duration_ms"`
	ErrorMessage     string    `json:"error_message"`
	CreatedAt        time.Time `json:"created_at"`
}

func scanToken(row interface{ Scan(dest ...any) error }) (*Token, error) {
	var t Token
	var usedAt, refreshedAt, testedAt, quotaAt sql.NullTime
	var enabled int
	err := row.Scan(&t.ID, &t.Label, &t.AccessToken, &t.RefreshToken, &t.UserInfo,
		&t.Status, &enabled, &t.QuotaInfo, &usedAt, &refreshedAt, &testedAt, &quotaAt,
		&t.TestLatencyMs, &t.ErrorMessage, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	t.Enabled = enabled == 1
	if usedAt.Valid {
		t.LastUsedAt = &usedAt.Time
	}
	if refreshedAt.Valid {
		t.LastRefreshedAt = &refreshedAt.Time
	}
	if testedAt.Valid {
		t.LastTestedAt = &testedAt.Time
	}
	if quotaAt.Valid {
		t.LastQuotaAt = &quotaAt.Time
	}
	return &t, nil
}

func scanAPIKey(row interface{ Scan(dest ...any) error }) (*APIKey, error) {
	var k APIKey
	var expiresAt sql.NullTime
	var enabled int
	err := row.Scan(&k.ID, &k.Name, &k.Key, &enabled, &k.QuotaLimit, &k.QuotaUsed, &expiresAt, &k.CreatedAt)
	if err != nil {
		return nil, err
	}
	k.Enabled = enabled == 1
	if expiresAt.Valid {
		k.ExpiresAt = &expiresAt.Time
	}
	return &k, nil
}

func scanUsageLog(row interface{ Scan(dest ...any) error }) (*UsageLog, error) {
	var l UsageLog
	var isStream int
	var errMsg sql.NullString
	err := row.Scan(&l.ID, &l.TokenID, &l.APIKeyID, &l.Model, &l.Endpoint, &isStream,
		&l.PromptTokens, &l.CompletionTokens, &l.TotalTokens, &l.StatusCode,
		&l.DurationMs, &errMsg, &l.CreatedAt)
	if err != nil {
		return nil, err
	}
	l.IsStream = isStream == 1
	if errMsg.Valid {
		l.ErrorMessage = errMsg.String
	}
	return &l, nil
}

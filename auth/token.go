package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/atom2api/atom2api/database"
)

type TokenPool struct {
	counter atomic.Int64
	db      *database.DB
}

func NewTokenPool(db *database.DB) *TokenPool {
	return &TokenPool{db: db}
}

func (p *TokenPool) Select() (*database.Token, error) {
	tokens, err := p.db.GetActiveTokens()
	if err != nil {
		return nil, fmt.Errorf("query active tokens: %w", err)
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no active tokens available")
	}
	idx := p.counter.Add(1) - 1
	t := tokens[int(idx)%len(tokens)]
	_ = p.db.TouchToken(t.ID)
	return &t, nil
}

// ImportFromEnv imports tokens from ATOMCODE_TOKENS env var.
// Format: "access_token1:refresh_token1,access_token2:refresh_token2"
// Or just: "access_token1,access_token2" (no refresh tokens)
func ImportFromEnv(db *database.DB) (int, error) {
	raw := os.Getenv("ATOMCODE_TOKENS")
	if raw == "" {
		return 0, nil
	}
	imported := 0
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, ":", 2)
		accessToken := strings.TrimSpace(parts[0])
		refreshToken := ""
		if len(parts) == 2 {
			refreshToken = strings.TrimSpace(parts[1])
		}
		if accessToken == "" {
			continue
		}
		exists, err := db.TokenExists(accessToken)
		if err != nil {
			return imported, err
		}
		if exists {
			continue
		}
		_, err = db.InsertToken("env-import", accessToken, refreshToken, "{}")
		if err != nil {
			return imported, err
		}
		imported++
	}
	return imported, nil
}

func RefreshOneToken(ctx context.Context, db *database.DB, tokenID int64) error {
	tokens, err := db.ListTokens()
	if err != nil {
		return err
	}
	var tok *database.Token
	for _, t := range tokens {
		if t.ID == tokenID {
			tok = &t
			break
		}
	}
	if tok == nil {
		return fmt.Errorf("token %d not found", tokenID)
	}
	if tok.RefreshToken == "" {
		return fmt.Errorf("token %d has no refresh_token", tokenID)
	}
	return doRefresh(ctx, db, tok)
}

func doRefresh(ctx context.Context, db *database.DB, tok *database.Token) error {
	body, _ := json.Marshal(map[string]string{"refresh_token": tok.RefreshToken})
	req, err := http.NewRequestWithContext(ctx, "POST", "https://acs.atomgit.com/oauth/refresh", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		_ = db.UpdateTokenStatus(tok.ID, "error", err.Error())
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		errMsg := fmt.Sprintf("refresh failed: HTTP %d", resp.StatusCode)
		_ = db.UpdateTokenStatus(tok.ID, "error", errMsg)
		return fmt.Errorf(errMsg)
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		_ = db.UpdateTokenStatus(tok.ID, "error", "failed to parse refresh response")
		return err
	}
	if result.AccessToken == "" {
		_ = db.UpdateTokenStatus(tok.ID, "error", "empty access_token in refresh response")
		return fmt.Errorf("empty access_token")
	}
	newRT := result.RefreshToken
	if newRT == "" {
		newRT = tok.RefreshToken
	}
	return db.UpdateTokenTokens(tok.ID, result.AccessToken, newRT)
}

func StartRefreshLoop(ctx context.Context, db *database.DB, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				refreshAll(ctx, db)
			}
		}
	}()
}

func refreshAll(ctx context.Context, db *database.DB) {
	autoRefresh, _ := db.GetSetting("auto_refresh_tokens")
	if autoRefresh != "true" {
		return
	}
	tokens, err := db.ListTokens()
	if err != nil {
		return
	}
	for _, t := range tokens {
		if t.RefreshToken == "" {
			continue
		}
		_ = doRefresh(ctx, db, &t)
	}
}

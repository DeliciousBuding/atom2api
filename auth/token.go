package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
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

var (
	tomlAccessTokenRE  = regexp.MustCompile(`(?m)^\s*access_token\s*=\s*"([^"]+)"`)
	tomlRefreshTokenRE = regexp.MustCompile(`(?m)^\s*refresh_token\s*=\s*"([^"]+)"`)
	tomlUsernameRE     = regexp.MustCompile(`(?m)^\s*username\s*=\s*"([^"]+)"`)
	tomlEmailRE        = regexp.MustCompile(`(?m)^\s*email\s*=\s*"([^"]+)"`)
)

// ImportFromTOML parses one or more auth.toml blocks (separated by blank lines or "---")
// and inserts each as a token row. Returns count imported and per-block errors.
func ImportFromTOML(db *database.DB, raw string) (int, []string) {
	blocks := splitTOMLBlocks(raw)
	imported := 0
	errs := []string{}
	for i, block := range blocks {
		atMatch := tomlAccessTokenRE.FindStringSubmatch(block)
		if len(atMatch) < 2 {
			errs = append(errs, fmt.Sprintf("block %d: no access_token found", i+1))
			continue
		}
		accessToken := atMatch[1]
		refreshToken := ""
		if m := tomlRefreshTokenRE.FindStringSubmatch(block); len(m) >= 2 {
			refreshToken = m[1]
		}
		label := ""
		if m := tomlUsernameRE.FindStringSubmatch(block); len(m) >= 2 {
			label = m[1]
		}
		userInfo := map[string]string{}
		if label != "" {
			userInfo["username"] = label
		}
		if m := tomlEmailRE.FindStringSubmatch(block); len(m) >= 2 {
			userInfo["email"] = m[1]
			if label == "" {
				label = m[1]
			}
		}
		exists, err := db.TokenExists(accessToken)
		if err != nil {
			errs = append(errs, fmt.Sprintf("block %d: %v", i+1, err))
			continue
		}
		if exists {
			errs = append(errs, fmt.Sprintf("block %d: duplicate access_token", i+1))
			continue
		}
		uiJSON, _ := json.Marshal(userInfo)
		if _, err := db.InsertToken(label, accessToken, refreshToken, string(uiJSON)); err != nil {
			errs = append(errs, fmt.Sprintf("block %d: insert failed: %v", i+1, err))
			continue
		}
		imported++
	}
	return imported, errs
}

func splitTOMLBlocks(raw string) []string {
	if strings.Contains(raw, "---") {
		out := []string{}
		for _, b := range strings.Split(raw, "---") {
			b = strings.TrimSpace(b)
			if b != "" {
				out = append(out, b)
			}
		}
		return out
	}
	if strings.Contains(raw, "\n\n") {
		out := []string{}
		for _, b := range strings.Split(raw, "\n\n") {
			b = strings.TrimSpace(b)
			if b != "" && tomlAccessTokenRE.MatchString(b) {
				out = append(out, b)
			}
		}
		if len(out) > 1 {
			return out
		}
	}
	return []string{raw}
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

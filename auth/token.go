package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/atom2api/atom2api/database"
	toml "github.com/pelletier/go-toml/v2"
)

type TokenData struct {
	AccessToken  string `toml:"access_token"`
	RefreshToken string `toml:"refresh_token"`
	TokenType    string `toml:"token_type"`
	ExpiresIn    int    `toml:"expires_in"`
	CreatedAt    int64  `toml:"created_at"`
}

type AtomCodeAuth struct {
	Token *TokenData `toml:"token"`
	User  struct {
		Username string `toml:"username"`
		Email    string `toml:"email"`
	} `toml:"user"`
}

type TokenPool struct {
	mu      sync.RWMutex
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

func ParseAtomCodeAuthDir(dir string) ([]AtomCodeAuth, error) {
	authFile := filepath.Join(dir, "auth.toml")
	data, err := os.ReadFile(authFile)
	if err != nil {
		return nil, fmt.Errorf("read auth.toml: %w", err)
	}
	var auth AtomCodeAuth
	if err := toml.Unmarshal(data, &auth); err != nil {
		return nil, fmt.Errorf("parse auth.toml: %w", err)
	}
	if auth.Token == nil || auth.Token.AccessToken == "" {
		return nil, fmt.Errorf("no access_token found in auth.toml")
	}
	return []AtomCodeAuth{auth}, nil
}

func ImportFromAtomCode(db *database.DB, dir string) (int, error) {
	auths, err := ParseAtomCodeAuthDir(dir)
	if err != nil {
		return 0, err
	}
	imported := 0
	for _, a := range auths {
		exists, err := db.TokenExists(a.Token.AccessToken)
		if err != nil {
			return imported, err
		}
		if exists {
			continue
		}
		label := a.User.Username
		if label == "" {
			label = a.User.Email
		}
		userInfo, _ := json.Marshal(map[string]string{
			"username": a.User.Username,
			"email":    a.User.Email,
		})
		_, err = db.InsertToken(label, a.Token.AccessToken, a.Token.RefreshToken, string(userInfo))
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

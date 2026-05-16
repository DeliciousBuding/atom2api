package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/atom2api/atom2api/auth"
	"github.com/atom2api/atom2api/config"
	"github.com/atom2api/atom2api/database"
)

type Handler struct {
	db     *database.DB
	pool   *auth.TokenPool
	cfg    *config.Config
}

func NewHandler(db *database.DB, pool *auth.TokenPool, cfg *config.Config) *Handler {
	return &Handler{db: db, pool: pool, cfg: cfg}
}

// BootstrapStatus returns whether admin secret needs to be set up.
func (h *Handler) HandleBootstrapStatus(w http.ResponseWriter, r *http.Request) {
	source := "empty"
	needsBootstrap := true
	if h.cfg.AdminSecret != "" {
		source = "env"
		needsBootstrap = false
	} else {
		dbSecret, _ := h.db.GetSetting("admin_secret")
		if dbSecret != "" {
			source = "database"
			needsBootstrap = false
		}
	}
	writeJSON(w, 200, map[string]any{
		"needs_bootstrap": needsBootstrap,
		"source":          source,
	})
}

// POST /api/admin/bootstrap — first-run admin secret setup (no auth required)
func (h *Handler) HandleBootstrap(w http.ResponseWriter, r *http.Request) {
	if h.cfg.AdminSecret != "" {
		writeJSON(w, 409, map[string]string{"error": "admin secret already set via environment variable"})
		return
	}
	dbSecret, _ := h.db.GetSetting("admin_secret")
	if dbSecret != "" {
		writeJSON(w, 409, map[string]string{"error": "admin secret already configured"})
		return
	}
	var req struct {
		AdminSecret string `json:"admin_secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.AdminSecret) < 4 {
		writeJSON(w, 400, map[string]string{"error": "admin_secret must be at least 4 characters"})
		return
	}
	if err := h.db.SetSetting("admin_secret", req.AdminSecret); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	h.cfg.AdminSecret = req.AdminSecret
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// POST /api/admin/auth/login
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Secret string `json:"secret"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Secret != h.cfg.AdminSecret {
		writeJSON(w, 401, map[string]string{"error": "invalid secret"})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_secret",
		Value:    h.cfg.AdminSecret,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400 * 7,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// GET /api/admin/stats
func (h *Handler) HandleStats(w http.ResponseWriter, r *http.Request) {
	total, active, errCount, _ := h.db.TokenCount()
	reqCount, tokenCount, _ := h.db.UsageStats()
	apiKeys, _ := h.db.ListAPIKeys()
	writeJSON(w, 200, map[string]any{
		"tokens_total":    total,
		"tokens_active":   active,
		"tokens_error":    errCount,
		"total_requests":  reqCount,
		"total_tokens":    tokenCount,
		"api_keys_count":  len(apiKeys),
	})
}

// GET /api/admin/tokens
func (h *Handler) HandleListTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.db.ListTokens()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	// Mask access tokens for security
	for i := range tokens {
		if len(tokens[i].AccessToken) > 16 {
			tokens[i].AccessToken = tokens[i].AccessToken[:16] + "..."
		}
	}
	writeJSON(w, 200, tokens)
}

// POST /api/admin/tokens
func (h *Handler) HandleAddToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Label        string `json:"label"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	if req.AccessToken == "" {
		writeJSON(w, 400, map[string]string{"error": "access_token required"})
		return
	}
	id, err := h.db.InsertToken(req.Label, req.AccessToken, req.RefreshToken, "{}")
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "status": "active"})
}

// POST /api/admin/tokens/import-env
func (h *Handler) HandleImportFromEnv(w http.ResponseWriter, r *http.Request) {
	imported, err := auth.ImportFromEnv(h.db)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error(), "imported": imported})
		return
	}
	writeJSON(w, 200, map[string]any{"imported": imported})
}

// POST /api/admin/tokens/import-toml — paste auth.toml content (one or more, separated by blank lines or ---)
func (h *Handler) HandleImportTOML(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TOML string `json:"toml"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	imported, errs := auth.ImportFromTOML(h.db, req.TOML)
	resp := map[string]any{"imported": imported}
	if len(errs) > 0 {
		resp["errors"] = errs
	}
	writeJSON(w, 200, resp)
}

// PATCH /api/admin/tokens/{id}/enable — body {"enabled": bool}
func (h *Handler) HandleToggleTokenEnabled(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	if err := h.db.SetTokenEnabled(id, req.Enabled); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"id": id, "enabled": req.Enabled})
}

// POST /api/admin/tokens/{id}/test — probe upstream with this token
func (h *Handler) HandleTestToken(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	tok, err := h.db.GetToken(id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "token not found"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	latency, err := probeToken(ctx, tok.AccessToken, h.cfg.UpstreamURL)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	_ = h.db.RecordTokenTest(tok.ID, int(latency.Milliseconds()), err == nil, errMsg)
	writeJSON(w, 200, map[string]any{
		"id":         tok.ID,
		"ok":         err == nil,
		"latency_ms": latency.Milliseconds(),
		"error":      errMsg,
	})
}

// POST /api/admin/tokens/batch-test — test all tokens concurrently
func (h *Handler) HandleBatchTestTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.db.ListTokens()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	type result struct {
		ID        int64  `json:"id"`
		OK        bool   `json:"ok"`
		LatencyMs int64  `json:"latency_ms"`
		Error     string `json:"error,omitempty"`
	}
	results := make([]result, len(tokens))
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	sem := make(chan struct{}, 5)
	done := make(chan int, len(tokens))
	for i, tok := range tokens {
		sem <- struct{}{}
		go func(idx int, t database.Token) {
			defer func() { <-sem; done <- 1 }()
			lat, perr := probeToken(ctx, t.AccessToken, h.cfg.UpstreamURL)
			errStr := ""
			if perr != nil {
				errStr = perr.Error()
			}
			_ = h.db.RecordTokenTest(t.ID, int(lat.Milliseconds()), perr == nil, errStr)
			results[idx] = result{ID: t.ID, OK: perr == nil, LatencyMs: lat.Milliseconds(), Error: errStr}
		}(i, tok)
	}
	for range tokens {
		<-done
	}
	writeJSON(w, 200, map[string]any{"results": results, "total": len(tokens)})
}

// POST /api/admin/tokens/{id}/quota — fetch CodingPlan status for this token
func (h *Handler) HandleTokenQuota(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	tok, err := h.db.GetToken(id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "token not found"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	quotaJSON, err := fetchQuota(ctx, tok.AccessToken, h.cfg.UpstreamStatus)
	if err != nil {
		writeJSON(w, 502, map[string]string{"error": err.Error()})
		return
	}
	_ = h.db.UpdateTokenQuota(tok.ID, quotaJSON)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write([]byte(quotaJSON))
}

func probeToken(ctx context.Context, accessToken, upstreamURL string) (time.Duration, error) {
	body := []byte(`{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"hi"}],"max_tokens":1,"stream":false}`)
	req, err := http.NewRequestWithContext(ctx, "POST", upstreamURL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", "atomcode/4.22.0")
	start := time.Now()
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	latency := time.Since(start)
	if err != nil {
		return latency, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		buf, _ := io.ReadAll(resp.Body)
		msg := strings.TrimSpace(string(buf))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return latency, fmt.Errorf("HTTP %d: %s", resp.StatusCode, msg)
	}
	io.Copy(io.Discard, resp.Body)
	return latency, nil
}

func fetchQuota(ctx context.Context, accessToken, statusURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", statusURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", "atomcode/4.22.0")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(buf))
	}
	return string(buf), nil
}

// POST /api/admin/tokens/{id}/refresh
func (h *Handler) HandleRefreshToken(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid token id"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := auth.RefreshOneToken(ctx, h.db, id); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "refreshed"})
}

// DELETE /api/admin/tokens/{id}
func (h *Handler) HandleDeleteToken(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := h.db.DeleteToken(id); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "deleted"})
}

// GET /api/admin/apikeys
func (h *Handler) HandleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.db.ListAPIKeys()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, keys)
}

// POST /api/admin/apikeys
func (h *Handler) HandleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string `json:"name"`
		QuotaLimit int64  `json:"quota_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	key, err := auth.CreateAPIKey(h.db, req.Name, req.QuotaLimit, nil)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]any{"key": key, "name": req.Name})
}

// DELETE /api/admin/apikeys/{id}
func (h *Handler) HandleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := h.db.DeleteAPIKey(id); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "deleted"})
}

// GET /api/admin/usage
func (h *Handler) HandleUsage(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)
	model := r.URL.Query().Get("model")
	logs, total, err := h.db.ListUsageLogs(limit, offset, model)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"logs": logs, "total": total})
}

// GET /api/admin/settings
func (h *Handler) HandleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.db.AllSettings()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, settings)
}

// PUT /api/admin/settings
func (h *Handler) HandleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	for k, v := range req {
		if err := h.db.SetSetting(k, v); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
	}
	writeJSON(w, 200, map[string]string{"status": "updated"})
}

func writeJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

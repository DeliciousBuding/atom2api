package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
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

// POST /api/admin/tokens/import-atomcode
func (h *Handler) HandleImportAtomCode(w http.ResponseWriter, r *http.Request) {
	dir := h.cfg.AtomCodeConfDir
	imported, err := auth.ImportFromAtomCode(h.db, dir)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error(), "imported": imported})
		return
	}
	writeJSON(w, 200, map[string]any{"imported": imported})
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

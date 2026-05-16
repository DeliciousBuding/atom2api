package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/atom2api/atom2api/auth"
	"github.com/atom2api/atom2api/database"
)

var DefaultModels = []map[string]any{
	{"id": "deepseek-v4-flash", "object": "model", "owned_by": "deepseek", "reasoning": true},
	{"id": "Qwen/Qwen3.6-35B-A3B", "object": "model", "owned_by": "qwen", "reasoning": true},
	{"id": "Qwen/Qwen3-32B", "object": "model", "owned_by": "qwen", "reasoning": true},
	{"id": "Qwen/Qwen3-30B-A3B", "object": "model", "owned_by": "qwen", "reasoning": true},
	{"id": "Qwen/Qwen3-VL-8B-Instruct", "object": "model", "owned_by": "qwen", "reasoning": false},
	{"id": "Qwen/Qwen3-Coder-480B-A35B-Instruct", "object": "model", "owned_by": "qwen", "reasoning": true},
}

var blockedModels = map[string]bool{"GLM-5.1": true, "glm-5.1": true}

type Handler struct {
	pool     *auth.TokenPool
	db       *database.DB
	upstream string
}

func NewHandler(pool *auth.TokenPool, db *database.DB, upstream string) *Handler {
	return &Handler{pool: pool, db: db, upstream: upstream}
}

func (h *Handler) HandleChatCompletions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	body, err := readBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var req struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	_ = json.Unmarshal(body, &req)

	if blockedModels[req.Model] {
		writeError(w, http.StatusForbidden, fmt.Sprintf("model %s requires pro plan", req.Model))
		return
	}

	tok, err := h.pool.Select()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "no available tokens: "+err.Error())
		return
	}

	resp, err := ForwardRequest(r.Context(), &UpstreamRequest{
		Body:   body,
		Token:  tok.AccessToken,
		URL:    h.upstream,
		Stream: req.Stream,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream error: "+err.Error())
		go h.logUsage(tok.ID, 0, req.Model, "/v1/chat/completions", req.Stream, 0, 0, 0, 0, int(time.Since(start).Milliseconds()), err.Error())
		return
	}
	defer func() {
		if !req.Stream {
			resp.Body.Close()
		}
	}()

	if resp.StatusCode >= 400 {
		errBody, _ := readBodyFromResp(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		w.Write(errBody)
		go h.logUsage(tok.ID, 0, req.Model, "/v1/chat/completions", req.Stream, 0, 0, 0, resp.StatusCode, int(time.Since(start).Milliseconds()), string(errBody))
		return
	}

	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(200)
		PipeSSE(w, resp.Body)
		resp.Body.Close()
		go h.logUsage(tok.ID, 0, req.Model, "/v1/chat/completions", true, 0, 0, 0, 200, int(time.Since(start).Milliseconds()), "")
	} else {
		respBody, _ := readBodyFromResp(resp)
		pt, ct, tt := ExtractUsageFromBody(respBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
		go h.logUsage(tok.ID, 0, req.Model, "/v1/chat/completions", false, pt, ct, tt, resp.StatusCode, int(time.Since(start).Milliseconds()), "")
	}
}

func (h *Handler) HandleListModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	data := map[string]any{
		"object": "list",
		"data":   DefaultModels,
	}
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	total, active, errCount, _ := h.db.TokenCount()
	json.NewEncoder(w).Encode(map[string]any{
		"status":          "ok",
		"tokens_total":    total,
		"tokens_active":   active,
		"tokens_error":    errCount,
		"default_model":   "deepseek-v4-flash",
	})
}

func (h *Handler) logUsage(tokenID, apiKeyID int64, model, endpoint string, isStream bool, pt, ct, tt, statusCode, durationMs int, errMsg string) {
	_ = h.db.InsertUsageLog(&database.UsageLog{
		TokenID:          tokenID,
		APIKeyID:         apiKeyID,
		Model:            model,
		Endpoint:         endpoint,
		IsStream:         isStream,
		PromptTokens:     pt,
		CompletionTokens: ct,
		TotalTokens:      tt,
		StatusCode:       statusCode,
		DurationMs:       durationMs,
		ErrorMessage:     errMsg,
	})
}

func readBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	buf := make([]byte, r.ContentLength)
	_, err := r.Body.Read(buf)
	if err != nil && err.Error() != "EOF" {
		return nil, err
	}
	return buf, nil
}

func readBodyFromResp(resp *http.Response) ([]byte, error) {
	buf := make([]byte, 0, 64*1024)
	tmp := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	return buf, nil
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"message": msg,
			"type":    "atom2api_error",
			"code":    strings.ReplaceAll(strings.ToLower(http.StatusText(code)), " ", "_"),
		},
	})
}

func readBodyLimited(r *http.Request, limit int64) ([]byte, error) {
	defer r.Body.Close()
	buf := make([]byte, limit)
	n, err := r.Body.Read(buf)
	if err != nil && err.Error() != "EOF" && n == 0 {
		return nil, err
	}
	log.Printf("read %d bytes", n)
	return buf[:n], nil
}

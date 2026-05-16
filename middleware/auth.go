package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/atom2api/atom2api/database"
)

func APIKeyAuth(db *database.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hasKeys, _ := db.HasAPIKeys()
		if !hasKeys {
			next.ServeHTTP(w, r)
			return
		}
		key := extractAPIKey(r)
		if key == "" {
			writeAuthError(w, "missing API key")
			return
		}
		ak, err := db.ValidateAPIKey(key)
		if err != nil {
			writeAuthError(w, "invalid API key: "+err.Error())
			return
		}
		r.Header.Set("X-Atom2API-Key-ID", fmt.Sprintf("%d", ak.ID))
		next.ServeHTTP(w, r)
	})
}

func AdminAuth(adminSecret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if adminSecret == "" {
			next.ServeHTTP(w, r)
			return
		}
		secret := r.Header.Get("X-Admin-Secret")
		if secret == "" {
			cookie, err := r.Cookie("admin_secret")
			if err == nil {
				secret = cookie.Value
			}
		}
		if secret != adminSecret {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func extractAPIKey(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer ")
		}
	}
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}
	return ""
}

func writeAuthError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(401)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"message": msg,
			"type":    "authentication_error",
			"code":    "invalid_api_key",
		},
	})
}

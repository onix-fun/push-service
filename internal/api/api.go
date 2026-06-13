package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/onix-fun/push-service/internal/config"
	"github.com/onix-fun/push-service/internal/model"
	"github.com/onix-fun/push-service/internal/store"
)

func Handler(s *store.Store, keys []config.APIKey) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /v1/devices/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !authorize(w, r, keys, "devices:write") {
			return
		}
		var device model.Device
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&device); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		device.ID = r.PathValue("id")
		if device.ID == "" || device.RecipientID == "" || device.Token == "" || !validProvider(device.Provider) {
			http.Error(w, "id, recipient_id, provider and token are required", http.StatusBadRequest)
			return
		}
		if err := s.UpsertDevice(r.Context(), device); err != nil {
			http.Error(w, "database unavailable", 503)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("DELETE /v1/devices/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !authorize(w, r, keys, "devices:write") {
			return
		}
		if err := s.DeactivateDevice(r.Context(), r.PathValue("id")); err != nil {
			http.Error(w, "database unavailable", 503)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /v1/recipients/{recipient}/devices", func(w http.ResponseWriter, r *http.Request) {
		if !authorize(w, r, keys, "devices:read") {
			return
		}
		devices, err := s.Devices(r.Context(), r.PathValue("recipient"))
		if err != nil {
			http.Error(w, "database unavailable", 503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"devices": devices})
	})
	return mux
}

func validProvider(value string) bool {
	return value == "fcm" || value == "apns" || value == "web_push"
}

func authorize(w http.ResponseWriter, r *http.Request, keys []config.APIKey, scope string) bool {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	for _, key := range keys {
		if subtle.ConstantTimeCompare([]byte(token), []byte(key.Value)) == 1 {
			for _, allowed := range key.Scopes {
				if allowed == scope || allowed == "*" {
					return true
				}
			}
		}
	}
	http.Error(w, "forbidden", http.StatusForbidden)
	return false
}

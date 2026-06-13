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
	h := &handler{s: s, keys: keys}
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /v1/devices/{id}", h.UpsertDevice)
	mux.HandleFunc("DELETE /v1/devices/{id}", h.DeactivateDevice)
	mux.HandleFunc("GET /v1/recipients/{recipient}/devices", h.ListDevices)
	return mux
}

type handler struct {
	s    *store.Store
	keys []config.APIKey
}

// UpsertDevice godoc
// @Summary Register or update a device
// @Description Registers a new device token or updates an existing one for push notifications.
// @Tags devices
// @Accept json
// @Param id path string true "Device ID"
// @Param device body model.Device true "Device details"
// @Success 204 "No Content"
// @Failure 400 {string} string "Invalid JSON or missing fields"
// @Security ApiKeyAuth
// @Router /v1/devices/{id} [put]
func (h *handler) UpsertDevice(w http.ResponseWriter, r *http.Request) {
	if !authorize(w, r, h.keys, "devices:write") {
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
	if err := h.s.UpsertDevice(r.Context(), device); err != nil {
		http.Error(w, "database unavailable", 503)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeactivateDevice godoc
// @Summary Deactivate a device
// @Description Marks a device as inactive so it no longer receives notifications.
// @Tags devices
// @Param id path string true "Device ID"
// @Success 204 "No Content"
// @Security ApiKeyAuth
// @Router /v1/devices/{id} [delete]
func (h *handler) DeactivateDevice(w http.ResponseWriter, r *http.Request) {
	if !authorize(w, r, h.keys, "devices:write") {
		return
	}
	if err := h.s.DeactivateDevice(r.Context(), r.PathValue("id")); err != nil {
		http.Error(w, "database unavailable", 503)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListDevices godoc
// @Summary List devices for a recipient
// @Description Returns a list of all devices registered for a specific recipient ID.
// @Tags recipients
// @Produce json
// @Param recipient path string true "Recipient ID"
// @Success 200 {object} map[string][]model.Device
// @Security ApiKeyAuth
// @Router /v1/recipients/{recipient}/devices [get]
func (h *handler) ListDevices(w http.ResponseWriter, r *http.Request) {
	if !authorize(w, r, h.keys, "devices:read") {
		return
	}
	devices, err := h.s.Devices(r.Context(), r.PathValue("recipient"))
	if err != nil {
		http.Error(w, "database unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"devices": devices})
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

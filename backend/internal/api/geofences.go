package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Archit-Kamble/geofence-alert-system/backend/internal/store"
)

type createGeofenceRequest struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Coordinates [][]float64 `json:"coordinates"`
	Category    string      `json:"category"`
}

type createGeofenceResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Timed
}

// POST /geofences
func (s *Server) createGeofence(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req createGeofenceRequest
	if err := decodeBody(w, r, &req); err != nil {
		writeError(w, start, http.StatusBadRequest, err.Error())
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, start, http.StatusBadRequest, "name is required")
		return
	}
	if !allowedCategories[req.Category] {
		writeError(w, start, http.StatusBadRequest,
			"category must be one of: delivery_zone, restricted_zone, toll_zone, customer_area")
		return
	}
	if err := validateCoordinates(req.Coordinates); err != nil {
		writeError(w, start, http.StatusBadRequest, err.Error())
		return
	}

	geofence, err := s.store.CreateGeofence(r.Context(), req.Name, strings.TrimSpace(req.Description),
		req.Category, req.Coordinates)
	if errors.Is(err, store.ErrInvalidPolygon) {
		writeError(w, start, http.StatusBadRequest,
			"coordinates do not form a valid polygon (the boundary must not cross itself)")
		return
	}
	if err != nil {
		serverError(w, start, err)
		return
	}

	writeJSON(w, start, http.StatusCreated, &createGeofenceResponse{
		ID:     formatID(prefixGeofence, geofence.ID),
		Name:   geofence.Name,
		Status: geofence.Status,
	})
}

type geofenceItem struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Coordinates [][]float64 `json:"coordinates"`
	Category    string      `json:"category"`
	CreatedAt   string      `json:"created_at"`
}

type listGeofencesResponse struct {
	Geofences []geofenceItem `json:"geofences"`
	Timed
}

// GET /geofences?category=delivery_zone
func (s *Server) listGeofences(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	category := r.URL.Query().Get("category")
	if category != "" && !allowedCategories[category] {
		writeError(w, start, http.StatusBadRequest,
			"category must be one of: delivery_zone, restricted_zone, toll_zone, customer_area")
		return
	}

	geofences, err := s.store.ListGeofences(r.Context(), category)
	if err != nil {
		serverError(w, start, err)
		return
	}

	items := make([]geofenceItem, len(geofences))
	for i, g := range geofences {
		items[i] = geofenceItem{
			ID:          formatID(prefixGeofence, g.ID),
			Name:        g.Name,
			Description: g.Description,
			Coordinates: g.Coordinates,
			Category:    g.Category,
			CreatedAt:   formatTime(g.CreatedAt),
		}
	}
	writeJSON(w, start, http.StatusOK, &listGeofencesResponse{Geofences: items})
}

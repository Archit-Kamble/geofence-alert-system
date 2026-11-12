package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Archit-Kamble/geofence-alert-system/backend/internal/store"
)

const (
	defaultHistoryLimit = 50
	maxHistoryLimit     = 500
)

type violationItem struct {
	ID            string  `json:"id"`
	VehicleID     string  `json:"vehicle_id"`
	VehicleNumber string  `json:"vehicle_number"`
	GeofenceID    string  `json:"geofence_id"`
	GeofenceName  string  `json:"geofence_name"`
	EventType     string  `json:"event_type"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Timestamp     string  `json:"timestamp"`
}

type violationHistoryResponse struct {
	Violations []violationItem `json:"violations"`
	TotalCount int             `json:"total_count"`
	Timed
}

// GET /violations/history?vehicle_id=&geofence_id=&start_date=&end_date=&limit=&offset=
// "offset" is an addition to the brief: with "limit" and "total_count" it gives pagination.
func (s *Server) violationHistory(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	query := r.URL.Query()

	filter := store.EventFilter{Limit: defaultHistoryLimit}
	var err error

	if filter.VehicleID, err = optionalID(r, "vehicle_id", prefixVehicle); err != nil {
		writeError(w, start, http.StatusBadRequest, err.Error())
		return
	}
	if filter.GeofenceID, err = optionalID(r, "geofence_id", prefixGeofence); err != nil {
		writeError(w, start, http.StatusBadRequest, err.Error())
		return
	}
	if filter.Start, err = optionalTime(query.Get("start_date"), "start_date"); err != nil {
		writeError(w, start, http.StatusBadRequest, err.Error())
		return
	}
	if filter.End, err = optionalTime(query.Get("end_date"), "end_date"); err != nil {
		writeError(w, start, http.StatusBadRequest, err.Error())
		return
	}
	if filter.Start != nil && filter.End != nil && filter.End.Before(*filter.Start) {
		writeError(w, start, http.StatusBadRequest, "end_date must not be before start_date")
		return
	}

	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 {
			writeError(w, start, http.StatusBadRequest, "limit must be a number between 1 and 500")
			return
		}
		if limit > maxHistoryLimit {
			limit = maxHistoryLimit // larger values are capped, not rejected
		}
		filter.Limit = limit
	}
	if raw := query.Get("offset"); raw != "" {
		offset, err := strconv.Atoi(raw)
		if err != nil || offset < 0 {
			writeError(w, start, http.StatusBadRequest, "offset must be 0 or a positive number")
			return
		}
		filter.Offset = offset
	}

	events, total, err := s.store.ListEvents(r.Context(), filter)
	if err != nil {
		serverError(w, start, err)
		return
	}

	items := make([]violationItem, len(events))
	for i, e := range events {
		items[i] = violationItem{
			ID:            formatID(prefixViolation, e.ID),
			VehicleID:     formatID(prefixVehicle, e.VehicleID),
			VehicleNumber: e.VehicleNumber,
			GeofenceID:    formatID(prefixGeofence, e.GeofenceID),
			GeofenceName:  e.GeofenceName,
			EventType:     e.EventType,
			Latitude:      e.Latitude,
			Longitude:     e.Longitude,
			Timestamp:     formatTime(e.OccurredAt),
		}
	}
	writeJSON(w, start, http.StatusOK, &violationHistoryResponse{Violations: items, TotalCount: total})
}

// optionalTime parses an ISO 8601 timestamp. An empty value means "no filter".
func optionalTime(value, name string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, errors.New(name + " must be in ISO 8601 format, for example 2025-01-01T00:00:00Z")
	}
	return &t, nil
}

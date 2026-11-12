package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/Archit-Kamble/geofence-alert-system/backend/internal/store"
)

type configureAlertRequest struct {
	GeofenceID string `json:"geofence_id"`
	VehicleID  string `json:"vehicle_id"` // optional: empty means all vehicles
	EventType  string `json:"event_type"`
}

type configureAlertResponse struct {
	AlertID    string  `json:"alert_id"`
	GeofenceID string  `json:"geofence_id"`
	VehicleID  *string `json:"vehicle_id"` // null when the rule is for all vehicles
	EventType  string  `json:"event_type"`
	Status     string  `json:"status"`
	Timed
}

// POST /alerts/configure
func (s *Server) configureAlert(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req configureAlertRequest
	if err := decodeBody(w, r, &req); err != nil {
		writeError(w, start, http.StatusBadRequest, err.Error())
		return
	}

	geofenceID, err := parseID(prefixGeofence, req.GeofenceID)
	if err != nil {
		writeError(w, start, http.StatusBadRequest, "geofence_id: "+err.Error())
		return
	}
	if !allowedEventTypes[req.EventType] {
		writeError(w, start, http.StatusBadRequest, "event_type must be one of: entry, exit, both")
		return
	}

	exists, err := s.store.GeofenceExists(r.Context(), geofenceID)
	if err != nil {
		serverError(w, start, err)
		return
	}
	if !exists {
		writeError(w, start, http.StatusNotFound, "geofence "+req.GeofenceID+" not found")
		return
	}

	// The vehicle is optional. When given, it must exist.
	var vehicleID *int64
	var vehicleIDText *string
	if req.VehicleID != "" {
		id, err := parseID(prefixVehicle, req.VehicleID)
		if err != nil {
			writeError(w, start, http.StatusBadRequest, "vehicle_id: "+err.Error())
			return
		}
		if _, err := s.store.GetVehicle(r.Context(), id); errors.Is(err, store.ErrNotFound) {
			writeError(w, start, http.StatusNotFound, "vehicle "+req.VehicleID+" not found")
			return
		} else if err != nil {
			serverError(w, start, err)
			return
		}
		vehicleID = &id
		vehicleIDText = &req.VehicleID
	}

	rule, err := s.store.CreateAlertRule(r.Context(), geofenceID, vehicleID, req.EventType)
	if errors.Is(err, store.ErrDuplicate) {
		writeError(w, start, http.StatusConflict, "this alert rule already exists")
		return
	}
	if err != nil {
		serverError(w, start, err)
		return
	}

	writeJSON(w, start, http.StatusCreated, &configureAlertResponse{
		AlertID:    formatID(prefixAlert, rule.ID),
		GeofenceID: req.GeofenceID,
		VehicleID:  vehicleIDText,
		EventType:  rule.EventType,
		Status:     rule.Status,
	})
}

type alertItem struct {
	AlertID       string  `json:"alert_id"`
	GeofenceID    string  `json:"geofence_id"`
	GeofenceName  string  `json:"geofence_name"`
	VehicleID     *string `json:"vehicle_id"`     // null: all vehicles
	VehicleNumber *string `json:"vehicle_number"` // null: all vehicles
	EventType     string  `json:"event_type"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"created_at"`
}

type listAlertsResponse struct {
	Alerts []alertItem `json:"alerts"`
	Timed
}

// GET /alerts?geofence_id=geo_1&vehicle_id=veh_2
func (s *Server) listAlerts(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	geofenceID, err := optionalID(r, "geofence_id", prefixGeofence)
	if err != nil {
		writeError(w, start, http.StatusBadRequest, err.Error())
		return
	}
	vehicleID, err := optionalID(r, "vehicle_id", prefixVehicle)
	if err != nil {
		writeError(w, start, http.StatusBadRequest, err.Error())
		return
	}

	rules, err := s.store.ListAlertRules(r.Context(), geofenceID, vehicleID)
	if err != nil {
		serverError(w, start, err)
		return
	}

	items := make([]alertItem, len(rules))
	for i, rule := range rules {
		item := alertItem{
			AlertID:       formatID(prefixAlert, rule.ID),
			GeofenceID:    formatID(prefixGeofence, rule.GeofenceID),
			GeofenceName:  rule.GeofenceName,
			VehicleNumber: rule.VehicleNumber,
			EventType:     rule.EventType,
			Status:        rule.Status,
			CreatedAt:     formatTime(rule.CreatedAt),
		}
		if rule.VehicleID != nil {
			id := formatID(prefixVehicle, *rule.VehicleID)
			item.VehicleID = &id
		}
		items[i] = item
	}
	writeJSON(w, start, http.StatusOK, &listAlertsResponse{Alerts: items})
}

type deleteAlertResponse struct {
	AlertID string `json:"alert_id"`
	Deleted bool   `json:"deleted"`
	Timed
}

// DELETE /alerts/{alert_id}
// Not in the brief. Added so that the UI can "manage" rules by removing them.
func (s *Server) deleteAlert(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	rawID := r.PathValue("alert_id")
	alertID, err := parseID(prefixAlert, rawID)
	if err != nil {
		writeError(w, start, http.StatusBadRequest, "alert_id: "+err.Error())
		return
	}

	err = s.store.DeleteAlertRule(r.Context(), alertID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, start, http.StatusNotFound, "alert "+rawID+" not found")
		return
	}
	if err != nil {
		serverError(w, start, err)
		return
	}
	writeJSON(w, start, http.StatusOK, &deleteAlertResponse{AlertID: rawID, Deleted: true})
}

// optionalID reads an id from the query string. It returns nil when the
// parameter is absent, and an error when it is present but malformed.
func optionalID(r *http.Request, name, prefix string) (*int64, error) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return nil, nil
	}
	id, err := parseID(prefix, value)
	if err != nil {
		return nil, errors.New(name + ": " + err.Error())
	}
	return &id, nil
}

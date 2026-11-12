package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/Archit-Kamble/geofence-alert-system/backend/internal/store"
)

// Latitude and longitude are pointers so that a missing field (nil) can be
// told apart from a real value of 0.
type updateLocationRequest struct {
	VehicleID string   `json:"vehicle_id"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Timestamp string   `json:"timestamp"`
}

type insideGeofence struct {
	GeofenceID   string `json:"geofence_id"`
	GeofenceName string `json:"geofence_name"`
	Status       string `json:"status"`
}

type updateLocationResponse struct {
	VehicleID        string           `json:"vehicle_id"`
	LocationUpdated  bool             `json:"location_updated"`
	CurrentGeofences []insideGeofence `json:"current_geofences"`
	Timed
}

// POST /vehicles/location
// Saves the location, detects entries and exits, and pushes alerts.
func (s *Server) updateLocation(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req updateLocationRequest
	if err := decodeBody(w, r, &req); err != nil {
		writeError(w, start, http.StatusBadRequest, err.Error())
		return
	}

	vehicleID, err := parseID(prefixVehicle, req.VehicleID)
	if err != nil {
		writeError(w, start, http.StatusBadRequest, "vehicle_id: "+err.Error())
		return
	}
	if req.Latitude == nil || !validLatitude(*req.Latitude) {
		writeError(w, start, http.StatusBadRequest, "latitude is required and must be between -90 and 90")
		return
	}
	if req.Longitude == nil || !validLongitude(*req.Longitude) {
		writeError(w, start, http.StatusBadRequest, "longitude is required and must be between -180 and 180")
		return
	}
	recordedAt, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		writeError(w, start, http.StatusBadRequest,
			"timestamp is required in ISO 8601 format, for example 2025-01-15T10:35:00Z")
		return
	}

	result, err := s.store.RecordLocation(r.Context(), vehicleID, *req.Latitude, *req.Longitude, recordedAt)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, start, http.StatusNotFound, "vehicle "+req.VehicleID+" not found")
		return
	}
	if err != nil {
		serverError(w, start, err)
		return
	}

	// Alerts are only queued here. The hub's own goroutine sends them to the
	// browsers, so this response does not wait for any WebSocket write.
	for _, event := range result.Events {
		if event.Alerted {
			s.hub.Publish(newAlertMessage(event))
		}
	}

	current := make([]insideGeofence, len(result.Current))
	for i, g := range result.Current {
		current[i] = insideGeofence{
			GeofenceID:   formatID(prefixGeofence, g.ID),
			GeofenceName: g.Name,
			Status:       "inside",
		}
	}
	writeJSON(w, start, http.StatusOK, &updateLocationResponse{
		VehicleID:        req.VehicleID,
		LocationUpdated:  true,
		CurrentGeofences: current,
	})
}

// alertMessage is the WebSocket message format from the assignment brief.
type alertMessage struct {
	EventID   string        `json:"event_id"`
	EventType string        `json:"event_type"`
	Timestamp string        `json:"timestamp"`
	Vehicle   alertVehicle  `json:"vehicle"`
	Geofence  alertGeofence `json:"geofence"`
	Location  alertLocation `json:"location"`
}

type alertVehicle struct {
	VehicleID     string `json:"vehicle_id"`
	VehicleNumber string `json:"vehicle_number"`
	DriverName    string `json:"driver_name"`
}

type alertGeofence struct {
	GeofenceID   string `json:"geofence_id"`
	GeofenceName string `json:"geofence_name"`
	Category     string `json:"category"`
}

type alertLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func newAlertMessage(e store.Event) alertMessage {
	return alertMessage{
		EventID:   formatID(prefixEvent, e.ID),
		EventType: e.Type,
		Timestamp: formatTime(e.OccurredAt),
		Vehicle: alertVehicle{
			VehicleID:     formatID(prefixVehicle, e.Vehicle.ID),
			VehicleNumber: e.Vehicle.VehicleNumber,
			DriverName:    e.Vehicle.DriverName,
		},
		Geofence: alertGeofence{
			GeofenceID:   formatID(prefixGeofence, e.Geofence.ID),
			GeofenceName: e.Geofence.Name,
			Category:     e.Geofence.Category,
		},
		Location: alertLocation{Latitude: e.Latitude, Longitude: e.Longitude},
	}
}

type currentLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timestamp string  `json:"timestamp"`
}

type currentGeofence struct {
	GeofenceID   string `json:"geofence_id"`
	GeofenceName string `json:"geofence_name"`
	Category     string `json:"category"`
}

type getLocationResponse struct {
	VehicleID        string            `json:"vehicle_id"`
	VehicleNumber    string            `json:"vehicle_number"`
	CurrentLocation  *currentLocation  `json:"current_location"` // null until the first update
	CurrentGeofences []currentGeofence `json:"current_geofences"`
	Timed
}

// GET /vehicles/location/{vehicle_id}
func (s *Server) getLocation(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	rawID := r.PathValue("vehicle_id")
	vehicleID, err := parseID(prefixVehicle, rawID)
	if err != nil {
		writeError(w, start, http.StatusBadRequest, "vehicle_id: "+err.Error())
		return
	}

	vehicle, err := s.store.GetVehicle(r.Context(), vehicleID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, start, http.StatusNotFound, "vehicle "+rawID+" not found")
		return
	}
	if err != nil {
		serverError(w, start, err)
		return
	}

	location, geofences, err := s.store.CurrentLocation(r.Context(), vehicleID)
	if err != nil {
		serverError(w, start, err)
		return
	}

	resp := &getLocationResponse{
		VehicleID:        rawID,
		VehicleNumber:    vehicle.VehicleNumber,
		CurrentGeofences: make([]currentGeofence, len(geofences)),
	}
	if location != nil {
		resp.CurrentLocation = &currentLocation{
			Latitude:  location.Latitude,
			Longitude: location.Longitude,
			Timestamp: formatTime(location.RecordedAt),
		}
	}
	for i, g := range geofences {
		resp.CurrentGeofences[i] = currentGeofence{
			GeofenceID:   formatID(prefixGeofence, g.ID),
			GeofenceName: g.Name,
			Category:     g.Category,
		}
	}
	writeJSON(w, start, http.StatusOK, resp)
}

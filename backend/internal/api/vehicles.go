package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Archit-Kamble/geofence-alert-system/backend/internal/store"
)

type createVehicleRequest struct {
	VehicleNumber string `json:"vehicle_number"`
	DriverName    string `json:"driver_name"`
	VehicleType   string `json:"vehicle_type"`
	Phone         string `json:"phone"`
}

type createVehicleResponse struct {
	ID            string `json:"id"`
	VehicleNumber string `json:"vehicle_number"`
	Status        string `json:"status"`
	Timed
}

// POST /vehicles
func (s *Server) createVehicle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req createVehicleRequest
	if err := decodeBody(w, r, &req); err != nil {
		writeError(w, start, http.StatusBadRequest, err.Error())
		return
	}

	req.VehicleNumber = strings.TrimSpace(req.VehicleNumber)
	req.DriverName = strings.TrimSpace(req.DriverName)
	req.VehicleType = strings.TrimSpace(req.VehicleType)
	req.Phone = strings.TrimSpace(req.Phone)

	// All four fields are required by the brief.
	required := []struct{ name, value string }{
		{"vehicle_number", req.VehicleNumber},
		{"driver_name", req.DriverName},
		{"vehicle_type", req.VehicleType},
		{"phone", req.Phone},
	}
	for _, field := range required {
		if field.value == "" {
			writeError(w, start, http.StatusBadRequest, field.name+" is required")
			return
		}
	}

	vehicle, err := s.store.CreateVehicle(r.Context(), req.VehicleNumber, req.DriverName, req.VehicleType, req.Phone)
	if errors.Is(err, store.ErrDuplicate) {
		writeError(w, start, http.StatusConflict, "a vehicle with number "+req.VehicleNumber+" is already registered")
		return
	}
	if err != nil {
		serverError(w, start, err)
		return
	}

	writeJSON(w, start, http.StatusCreated, &createVehicleResponse{
		ID:            formatID(prefixVehicle, vehicle.ID),
		VehicleNumber: vehicle.VehicleNumber,
		Status:        vehicle.Status,
	})
}

type vehicleItem struct {
	ID            string `json:"id"`
	VehicleNumber string `json:"vehicle_number"`
	DriverName    string `json:"driver_name"`
	VehicleType   string `json:"vehicle_type"`
	Phone         string `json:"phone"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
}

type listVehiclesResponse struct {
	Vehicles []vehicleItem `json:"vehicles"`
	Timed
}

// GET /vehicles
func (s *Server) listVehicles(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	vehicles, err := s.store.ListVehicles(r.Context())
	if err != nil {
		serverError(w, start, err)
		return
	}

	items := make([]vehicleItem, len(vehicles))
	for i, v := range vehicles {
		items[i] = vehicleItem{
			ID:            formatID(prefixVehicle, v.ID),
			VehicleNumber: v.VehicleNumber,
			DriverName:    v.DriverName,
			VehicleType:   v.VehicleType,
			Phone:         v.Phone,
			Status:        v.Status,
			CreatedAt:     formatTime(v.CreatedAt),
		}
	}
	writeJSON(w, start, http.StatusOK, &listVehiclesResponse{Vehicles: items})
}

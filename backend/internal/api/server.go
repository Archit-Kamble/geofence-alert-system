// Package api is the HTTP layer: it reads requests, validates input,
// calls the store, and writes JSON responses.
package api

import (
	"log"
	"net/http"
	"time"

	"github.com/Archit-Kamble/geofence-alert-system/backend/internal/store"
	"github.com/Archit-Kamble/geofence-alert-system/backend/internal/ws"
)

// Server holds what the handlers need. It receives the store and the hub from
// main (dependency injection) instead of creating them itself.
type Server struct {
	store          *store.Store
	hub            *ws.Hub
	allowedOrigins []string // websites allowed to call the API; "*" means any
}

func NewServer(st *store.Store, hub *ws.Hub, allowedOrigins []string) *Server {
	return &Server{store: st, hub: hub, allowedOrigins: allowedOrigins}
}

// Routes connects each endpoint from the assignment brief to its handler.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /geofences", s.createGeofence)
	mux.HandleFunc("GET /geofences", s.listGeofences)

	mux.HandleFunc("POST /vehicles", s.createVehicle)
	mux.HandleFunc("GET /vehicles", s.listVehicles)

	mux.HandleFunc("POST /vehicles/location", s.updateLocation)
	mux.HandleFunc("GET /vehicles/location/{vehicle_id}", s.getLocation)

	mux.HandleFunc("POST /alerts/configure", s.configureAlert)
	mux.HandleFunc("GET /alerts", s.listAlerts)
	mux.HandleFunc("DELETE /alerts/{alert_id}", s.deleteAlert) // extra: lets the UI remove a rule

	mux.HandleFunc("GET /violations/history", s.violationHistory)

	mux.HandleFunc("GET /ws/alerts", s.alertsSocket)
	mux.HandleFunc("GET /health", s.health)

	// Anything else gets a JSON 404 that still carries time_ns.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, time.Now(), http.StatusNotFound, "no such endpoint: "+r.Method+" "+r.URL.Path)
	})

	return s.cors(logRequests(mux))
}

// cors lets the frontend, which runs on a different domain, call this API.
// Browsers block cross-domain calls unless the server sends these headers.
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && s.originAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		// Before a POST with JSON the browser first asks permission with OPTIONS.
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) originAllowed(origin string) bool {
	for _, allowed := range s.allowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}

// logRequests prints one line per request.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start))
	})
}

type healthResponse struct {
	Status string `json:"status"`
	Timed
}

// health is used by the hosting platform to check the service is up.
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	writeJSON(w, start, http.StatusOK, &healthResponse{Status: "ok"})
}

// alertsSocket upgrades the connection to a WebSocket for live alerts.
func (s *Server) alertsSocket(w http.ResponseWriter, r *http.Request) {
	s.hub.ServeWS(w, r, func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == "" || s.originAllowed(origin) // no Origin header: not a browser (curl, tests)
	})
}

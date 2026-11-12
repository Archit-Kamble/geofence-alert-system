package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Timed adds the required "time_ns" field. Every response struct embeds it,
// so no response can be written without the execution time.
type Timed struct {
	TimeNS string `json:"time_ns"`
}

func (t *Timed) stamp(start time.Time) {
	t.TimeNS = strconv.FormatInt(time.Since(start).Nanoseconds(), 10)
}

// stamper is any response body that embeds Timed.
type stamper interface {
	stamp(start time.Time)
}

// writeJSON fills in time_ns and writes the response.
// "start" is the time taken at the top of the handler.
func writeJSON(w http.ResponseWriter, start time.Time, status int, body stamper) {
	body.stamp(start)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("api: write response: %v", err)
	}
}

// errorResponse is the shape of every error: a message plus time_ns.
type errorResponse struct {
	Error string `json:"error"`
	Timed
}

func writeError(w http.ResponseWriter, start time.Time, status int, message string) {
	writeJSON(w, start, status, &errorResponse{Error: message})
}

// serverError logs the real error and gives the client a safe message.
func serverError(w http.ResponseWriter, start time.Time, err error) {
	log.Printf("api: internal error: %v", err)
	writeError(w, start, http.StatusInternalServerError, "internal server error")
}

// decodeBody reads a JSON request body (at most 1 MB) into dst.
func decodeBody(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return fmt.Errorf("request body must be valid JSON: %v", err)
	}
	return nil
}

// IDs are numbers in the database and prefixed strings in the API, for example 12 <-> "geo_12".
const (
	prefixGeofence  = "geo"
	prefixVehicle   = "veh"
	prefixAlert     = "alert"
	prefixViolation = "viol"
	prefixEvent     = "evt"
)

func formatID(prefix string, id int64) string {
	return prefix + "_" + strconv.FormatInt(id, 10)
}

// parseID turns "veh_456" into 456. It fails when the prefix or number is wrong.
func parseID(prefix, value string) (int64, error) {
	number, found := strings.CutPrefix(value, prefix+"_")
	if !found {
		return 0, fmt.Errorf("%q is not a valid id, expected the form %s_<number>", value, prefix)
	}
	id, err := strconv.ParseInt(number, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%q is not a valid id, expected the form %s_<number>", value, prefix)
	}
	return id, nil
}

// formatTime writes timestamps as ISO 8601 in UTC, for example 2025-01-15T10:35:00Z.
func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

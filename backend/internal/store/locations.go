package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Event types stored in geofence_events.
const (
	EventEntry = "entry"
	EventExit  = "exit"
)

// CategoryRestricted is the category that always alerts on entry.
const CategoryRestricted = "restricted_zone"

// GeofenceRef is the small part of a geofence needed while tracking a vehicle.
type GeofenceRef struct {
	ID       int64
	Name     string
	Category string
}

// Event is one entry or exit that was detected and saved.
type Event struct {
	ID         int64
	Type       string // EventEntry or EventExit
	Vehicle    Vehicle
	Geofence   GeofenceRef
	Latitude   float64
	Longitude  float64
	OccurredAt time.Time
	Alerted    bool // true when the event must be pushed to connected clients
}

// LocationResult is what one location update produced.
type LocationResult struct {
	Current []GeofenceRef // geofences the vehicle is inside after this update
	Events  []Event       // entries and exits caused by this update
}

// DiffGeofences compares where the vehicle was with where it is now.
// A geofence only in "current" was entered; one only in "previous" was exited.
// It is a plain function with no database access, so it is easy to test.
func DiffGeofences(previous, current []GeofenceRef) (entered, exited []GeofenceRef) {
	wasInside := make(map[int64]bool, len(previous))
	for _, g := range previous {
		wasInside[g.ID] = true
	}
	isInside := make(map[int64]bool, len(current))
	for _, g := range current {
		isInside[g.ID] = true
	}

	for _, g := range current {
		if !wasInside[g.ID] {
			entered = append(entered, g)
		}
	}
	for _, g := range previous {
		if !isInside[g.ID] {
			exited = append(exited, g)
		}
	}
	return entered, exited
}

// RecordLocation saves a location update and works out entries and exits.
//
// Everything happens in one transaction, so either all of it is saved or none.
// The vehicle row is locked first (FOR UPDATE): if two updates for the same
// vehicle arrive together, the second waits for the first. Without the lock,
// both could read the same "previous" set and report the same entry twice.
func (s *Store) RecordLocation(ctx context.Context, vehicleID int64, latitude, longitude float64, recordedAt time.Time) (LocationResult, error) {
	var result LocationResult

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx) // does nothing after a successful Commit

	// 1. Lock the vehicle row. This also proves the vehicle exists.
	var vehicle Vehicle
	err = tx.QueryRow(ctx, `
		SELECT id, vehicle_number, driver_name FROM vehicles WHERE id = $1 FOR UPDATE`, vehicleID,
	).Scan(&vehicle.ID, &vehicle.VehicleNumber, &vehicle.DriverName)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, ErrNotFound
	}
	if err != nil {
		return result, err
	}

	// 2. Keep the location in the history table.
	if _, err = tx.Exec(ctx, `
		INSERT INTO vehicle_locations (vehicle_id, latitude, longitude, recorded_at)
		VALUES ($1, $2, $3, $4)`, vehicleID, latitude, longitude, recordedAt); err != nil {
		return result, err
	}

	// 3. Which geofences cover the new point? ST_MakePoint takes (longitude, latitude).
	//    ST_Covers counts a point exactly on the border as inside.
	//    The GiST index on "boundary" keeps this fast when there are many geofences.
	current, err := queryGeofenceRefs(ctx, tx, `
		SELECT id, name, category
		FROM geofences
		WHERE status = 'active'
		  AND ST_Covers(boundary, ST_SetSRID(ST_MakePoint($1, $2), 4326))
		ORDER BY id`, longitude, latitude)
	if err != nil {
		return result, err
	}

	// 4. Which geofences was the vehicle inside before this update?
	//    On a vehicle's first update this is empty, so every geofence it is in counts as an entry.
	previous, err := queryGeofenceRefs(ctx, tx, `
		SELECT g.id, g.name, g.category
		FROM vehicle_geofence_state st
		JOIN geofences g ON g.id = st.geofence_id
		WHERE st.vehicle_id = $1
		ORDER BY g.id`, vehicleID)
	if err != nil {
		return result, err
	}

	// 5. Compare the two sets and save what changed.
	entered, exited := DiffGeofences(previous, current)

	for _, g := range entered {
		if _, err = tx.Exec(ctx, `
			INSERT INTO vehicle_geofence_state (vehicle_id, geofence_id, entered_at)
			VALUES ($1, $2, $3)`, vehicleID, g.ID, recordedAt); err != nil {
			return result, err
		}
		event, err := saveEvent(ctx, tx, EventEntry, vehicle, g, latitude, longitude, recordedAt)
		if err != nil {
			return result, err
		}
		result.Events = append(result.Events, event)
	}

	for _, g := range exited {
		if _, err = tx.Exec(ctx, `
			DELETE FROM vehicle_geofence_state WHERE vehicle_id = $1 AND geofence_id = $2`,
			vehicleID, g.ID); err != nil {
			return result, err
		}
		event, err := saveEvent(ctx, tx, EventExit, vehicle, g, latitude, longitude, recordedAt)
		if err != nil {
			return result, err
		}
		result.Events = append(result.Events, event)
	}

	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	result.Current = current
	return result, nil
}

// saveEvent stores one entry or exit and decides whether it must raise an alert.
// An alert is raised when a configured rule matches the event. Entering a
// restricted zone always raises one, even when nobody configured a rule.
func saveEvent(ctx context.Context, tx pgx.Tx, eventType string, vehicle Vehicle, geofence GeofenceRef,
	latitude, longitude float64, occurredAt time.Time) (Event, error) {

	// A rule matches when it is for this geofence, for this vehicle or for all
	// vehicles (vehicle_id IS NULL), and for this event type or for "both".
	var ruleMatches bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM alert_rules
			WHERE geofence_id = $1
			  AND status = 'active'
			  AND (vehicle_id IS NULL OR vehicle_id = $2)
			  AND event_type IN ($3, 'both')
		)`, geofence.ID, vehicle.ID, eventType).Scan(&ruleMatches)
	if err != nil {
		return Event{}, err
	}

	event := Event{
		Type:       eventType,
		Vehicle:    vehicle,
		Geofence:   geofence,
		Latitude:   latitude,
		Longitude:  longitude,
		OccurredAt: occurredAt,
		Alerted:    ruleMatches || (geofence.Category == CategoryRestricted && eventType == EventEntry),
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO geofence_events (vehicle_id, geofence_id, event_type, latitude, longitude, occurred_at, alerted)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		vehicle.ID, geofence.ID, eventType, latitude, longitude, occurredAt, event.Alerted,
	).Scan(&event.ID)
	return event, err
}

// queryGeofenceRefs runs a query that returns (id, name, category) rows.
func queryGeofenceRefs(ctx context.Context, tx pgx.Tx, sql string, args ...any) ([]GeofenceRef, error) {
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refs := []GeofenceRef{}
	for rows.Next() {
		var g GeofenceRef
		if err := rows.Scan(&g.ID, &g.Name, &g.Category); err != nil {
			return nil, err
		}
		refs = append(refs, g)
	}
	return refs, rows.Err()
}

// Location is a vehicle's latest known position.
type Location struct {
	Latitude   float64
	Longitude  float64
	RecordedAt time.Time
}

// CurrentLocation returns a vehicle's latest position and the geofences it is inside.
// The location is nil when the vehicle has never sent an update.
func (s *Store) CurrentLocation(ctx context.Context, vehicleID int64) (*Location, []GeofenceRef, error) {
	var location *Location
	var l Location
	err := s.pool.QueryRow(ctx, `
		SELECT latitude, longitude, recorded_at
		FROM vehicle_locations
		WHERE vehicle_id = $1
		ORDER BY id DESC
		LIMIT 1`, vehicleID).Scan(&l.Latitude, &l.Longitude, &l.RecordedAt)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// no update yet: location stays nil
	case err != nil:
		return nil, nil, err
	default:
		location = &l
	}

	rows, err := s.pool.Query(ctx, `
		SELECT g.id, g.name, g.category
		FROM vehicle_geofence_state st
		JOIN geofences g ON g.id = st.geofence_id
		WHERE st.vehicle_id = $1
		ORDER BY g.id`, vehicleID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	geofences := []GeofenceRef{}
	for rows.Next() {
		var g GeofenceRef
		if err := rows.Scan(&g.ID, &g.Name, &g.Category); err != nil {
			return nil, nil, err
		}
		geofences = append(geofences, g)
	}
	return location, geofences, rows.Err()
}

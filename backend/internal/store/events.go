package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// EventFilter narrows the violation history. Nil fields are ignored.
type EventFilter struct {
	VehicleID  *int64
	GeofenceID *int64
	Start      *time.Time
	End        *time.Time
	Limit      int
	Offset     int
}

// EventRow is one line of the violation history.
type EventRow struct {
	ID            int64
	VehicleID     int64
	VehicleNumber string
	GeofenceID    int64
	GeofenceName  string
	EventType     string
	Latitude      float64
	Longitude     float64
	OccurredAt    time.Time
}

// ListEvents returns one page of history, newest first, plus the total
// number of events that match the filter (for pagination).
func (s *Store) ListEvents(ctx context.Context, f EventFilter) ([]EventRow, int, error) {
	// Build the WHERE clause from the filters that were given.
	// Values always go in as $1, $2... parameters, never inside the SQL text,
	// so user input cannot change the query (no SQL injection).
	conditions := []string{"TRUE"}
	args := []any{}
	add := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(condition, len(args)))
	}
	if f.VehicleID != nil {
		add("e.vehicle_id = $%d", *f.VehicleID)
	}
	if f.GeofenceID != nil {
		add("e.geofence_id = $%d", *f.GeofenceID)
	}
	if f.Start != nil {
		add("e.occurred_at >= $%d", *f.Start)
	}
	if f.End != nil {
		add("e.occurred_at <= $%d", *f.End)
	}
	where := strings.Join(conditions, " AND ")

	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM geofence_events e WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, f.Limit, f.Offset)
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT e.id, e.vehicle_id, v.vehicle_number, e.geofence_id, g.name,
		       e.event_type, e.latitude, e.longitude, e.occurred_at
		FROM geofence_events e
		JOIN vehicles v ON v.id = e.vehicle_id
		JOIN geofences g ON g.id = e.geofence_id
		WHERE %s
		ORDER BY e.occurred_at DESC, e.id DESC
		LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	events := []EventRow{}
	for rows.Next() {
		var e EventRow
		if err := rows.Scan(&e.ID, &e.VehicleID, &e.VehicleNumber, &e.GeofenceID, &e.GeofenceName,
			&e.EventType, &e.Latitude, &e.Longitude, &e.OccurredAt); err != nil {
			return nil, 0, err
		}
		events = append(events, e)
	}
	return events, total, rows.Err()
}

package store

import (
	"context"
	"time"
)

// AlertRule is one configured rule, joined with names for display.
type AlertRule struct {
	ID            int64
	GeofenceID    int64
	GeofenceName  string
	VehicleID     *int64  // nil means the rule applies to all vehicles
	VehicleNumber *string // nil when VehicleID is nil
	EventType     string  // "entry", "exit" or "both"
	Status        string
	CreatedAt     time.Time
}

// CreateAlertRule saves a rule. The geofence and vehicle must already exist.
func (s *Store) CreateAlertRule(ctx context.Context, geofenceID int64, vehicleID *int64, eventType string) (AlertRule, error) {
	rule := AlertRule{GeofenceID: geofenceID, VehicleID: vehicleID, EventType: eventType}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO alert_rules (geofence_id, vehicle_id, event_type)
		VALUES ($1, $2, $3)
		RETURNING id, status, created_at`,
		geofenceID, vehicleID, eventType,
	).Scan(&rule.ID, &rule.Status, &rule.CreatedAt)
	if isUniqueViolation(err) {
		return AlertRule{}, ErrDuplicate
	}
	return rule, err
}

// ListAlertRules returns the rules, newest first. A nil filter means "no filter".
func (s *Store) ListAlertRules(ctx context.Context, geofenceID, vehicleID *int64) ([]AlertRule, error) {
	// LEFT JOIN on vehicles because a rule for all vehicles has no vehicle row.
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.geofence_id, g.name, r.vehicle_id, v.vehicle_number,
		       r.event_type, r.status, r.created_at
		FROM alert_rules r
		JOIN geofences g ON g.id = r.geofence_id
		LEFT JOIN vehicles v ON v.id = r.vehicle_id
		WHERE ($1::bigint IS NULL OR r.geofence_id = $1)
		  AND ($2::bigint IS NULL OR r.vehicle_id = $2)
		ORDER BY r.id DESC`, geofenceID, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := []AlertRule{}
	for rows.Next() {
		var r AlertRule
		if err := rows.Scan(&r.ID, &r.GeofenceID, &r.GeofenceName, &r.VehicleID, &r.VehicleNumber,
			&r.EventType, &r.Status, &r.CreatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// DeleteAlertRule removes a rule, or returns ErrNotFound.
func (s *Store) DeleteAlertRule(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM alert_rules WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

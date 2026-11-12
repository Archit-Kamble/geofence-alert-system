package store

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// Geofence is one row of the geofences table.
type Geofence struct {
	ID          int64
	Name        string
	Description string
	Category    string
	Coordinates [][]float64 // [latitude, longitude] pairs, as the client sent them
	Status      string
	CreatedAt   time.Time
}

// polygonWKT turns [latitude, longitude] pairs into PostGIS text,
// for example "POLYGON((77.59 12.97, 77.60 12.97, ...))".
// PostGIS wants longitude first (x, y), so the two numbers are swapped here.
// This is the only place where the swap happens for polygons.
func polygonWKT(coordinates [][]float64) string {
	points := make([]string, len(coordinates))
	for i, pair := range coordinates {
		latitude, longitude := pair[0], pair[1]
		points[i] = strconv.FormatFloat(longitude, 'f', -1, 64) + " " +
			strconv.FormatFloat(latitude, 'f', -1, 64)
	}
	return "POLYGON((" + strings.Join(points, ", ") + "))"
}

// CreateGeofence saves a new geofence. The coordinates must already have passed
// the API's validation. PostGIS does one more check: the polygon must not cross itself.
func (s *Store) CreateGeofence(ctx context.Context, name, description, category string, coordinates [][]float64) (Geofence, error) {
	wkt := polygonWKT(coordinates)

	var valid bool
	if err := s.pool.QueryRow(ctx,
		`SELECT ST_IsValid(ST_GeomFromText($1, 4326))`, wkt).Scan(&valid); err != nil {
		return Geofence{}, err
	}
	if !valid {
		return Geofence{}, ErrInvalidPolygon
	}

	coordinatesJSON, err := json.Marshal(coordinates)
	if err != nil {
		return Geofence{}, err
	}

	g := Geofence{Name: name, Description: description, Category: category, Coordinates: coordinates}
	err = s.pool.QueryRow(ctx, `
		INSERT INTO geofences (name, description, category, coordinates, boundary)
		VALUES ($1, $2, $3, $4::jsonb, ST_GeomFromText($5, 4326))
		RETURNING id, status, created_at`,
		name, description, category, string(coordinatesJSON), wkt,
	).Scan(&g.ID, &g.Status, &g.CreatedAt)
	return g, err
}

// ListGeofences returns all geofences, newest first.
// An empty category means "no filter".
func (s *Store) ListGeofences(ctx context.Context, category string) ([]Geofence, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, description, category, coordinates, status, created_at
		FROM geofences
		WHERE $1 = '' OR category = $1
		ORDER BY id DESC`, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	geofences := []Geofence{}
	for rows.Next() {
		var g Geofence
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.Category,
			&g.Coordinates, &g.Status, &g.CreatedAt); err != nil {
			return nil, err
		}
		geofences = append(geofences, g)
	}
	return geofences, rows.Err()
}

// GeofenceExists reports whether a geofence with this id is stored.
func (s *Store) GeofenceExists(ctx context.Context, id int64) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM geofences WHERE id = $1)`, id).Scan(&exists)
	return exists, err
}

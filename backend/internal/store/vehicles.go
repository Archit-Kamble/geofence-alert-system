package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Vehicle is one row of the vehicles table.
type Vehicle struct {
	ID            int64
	VehicleNumber string
	DriverName    string
	VehicleType   string
	Phone         string
	Status        string
	CreatedAt     time.Time
}

// CreateVehicle registers a vehicle. The vehicle number must be unique.
func (s *Store) CreateVehicle(ctx context.Context, number, driver, vehicleType, phone string) (Vehicle, error) {
	v := Vehicle{VehicleNumber: number, DriverName: driver, VehicleType: vehicleType, Phone: phone}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO vehicles (vehicle_number, driver_name, vehicle_type, phone)
		VALUES ($1, $2, $3, $4)
		RETURNING id, status, created_at`,
		number, driver, vehicleType, phone,
	).Scan(&v.ID, &v.Status, &v.CreatedAt)
	if isUniqueViolation(err) {
		return Vehicle{}, ErrDuplicate
	}
	return v, err
}

// ListVehicles returns all vehicles, newest first.
func (s *Store) ListVehicles(ctx context.Context) ([]Vehicle, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, vehicle_number, driver_name, vehicle_type, phone, status, created_at
		FROM vehicles
		ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	vehicles := []Vehicle{}
	for rows.Next() {
		var v Vehicle
		if err := rows.Scan(&v.ID, &v.VehicleNumber, &v.DriverName, &v.VehicleType,
			&v.Phone, &v.Status, &v.CreatedAt); err != nil {
			return nil, err
		}
		vehicles = append(vehicles, v)
	}
	return vehicles, rows.Err()
}

// GetVehicle returns one vehicle, or ErrNotFound.
func (s *Store) GetVehicle(ctx context.Context, id int64) (Vehicle, error) {
	var v Vehicle
	err := s.pool.QueryRow(ctx, `
		SELECT id, vehicle_number, driver_name, vehicle_type, phone, status, created_at
		FROM vehicles WHERE id = $1`, id,
	).Scan(&v.ID, &v.VehicleNumber, &v.DriverName, &v.VehicleType, &v.Phone, &v.Status, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Vehicle{}, ErrNotFound
	}
	return v, err
}

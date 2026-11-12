package api

import (
	"errors"
	"fmt"
)

var allowedCategories = map[string]bool{
	"delivery_zone":   true,
	"restricted_zone": true,
	"toll_zone":       true,
	"customer_area":   true,
}

var allowedEventTypes = map[string]bool{
	"entry": true,
	"exit":  true,
	"both":  true,
}

func validLatitude(v float64) bool  { return v >= -90 && v <= 90 }
func validLongitude(v float64) bool { return v >= -180 && v <= 180 }

// validateCoordinates checks the rules from the assignment brief:
// [latitude, longitude] pairs, valid ranges, a closed polygon and at least 4 points.
func validateCoordinates(coordinates [][]float64) error {
	if len(coordinates) < 4 {
		return errors.New("coordinates must have at least 4 points (3 unique points plus the closing point)")
	}

	for i, pair := range coordinates {
		if len(pair) != 2 {
			return fmt.Errorf("coordinates[%d] must be a [latitude, longitude] pair", i)
		}
		if !validLatitude(pair[0]) {
			return fmt.Errorf("coordinates[%d]: latitude %v must be between -90 and 90", i, pair[0])
		}
		if !validLongitude(pair[1]) {
			return fmt.Errorf("coordinates[%d]: longitude %v must be between -180 and 180", i, pair[1])
		}
	}

	first, last := coordinates[0], coordinates[len(coordinates)-1]
	if first[0] != last[0] || first[1] != last[1] {
		return errors.New("first and last coordinates must be identical (closed polygon)")
	}

	// A polygon needs 3 different corners. The closing point is skipped while counting.
	unique := map[[2]float64]bool{}
	for _, pair := range coordinates[:len(coordinates)-1] {
		unique[[2]float64{pair[0], pair[1]}] = true
	}
	if len(unique) < 3 {
		return errors.New("coordinates must contain at least 3 unique points")
	}
	return nil
}

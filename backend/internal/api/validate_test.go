package api

import "testing"

func TestValidateCoordinates(t *testing.T) {
	square := [][]float64{{12.9, 77.5}, {13.0, 77.5}, {13.0, 77.6}, {12.9, 77.6}, {12.9, 77.5}}

	tests := []struct {
		name        string
		coordinates [][]float64
		wantValid   bool
	}{
		{"valid closed square", square, true},
		{"valid triangle (3 unique + closing)", [][]float64{{0, 0}, {0, 1}, {1, 0}, {0, 0}}, true},
		{"empty", nil, false},
		{"too few points", [][]float64{{0, 0}, {0, 1}, {0, 0}}, false},
		{"not closed", [][]float64{{0, 0}, {0, 1}, {1, 1}, {1, 0}}, false},
		{"latitude out of range", [][]float64{{91, 0}, {0, 1}, {1, 0}, {91, 0}}, false},
		{"longitude out of range", [][]float64{{0, 181}, {0, 1}, {1, 0}, {0, 181}}, false},
		{"pair with one number", [][]float64{{0}, {0, 1}, {1, 0}, {0}}, false},
		{"pair with three numbers", [][]float64{{0, 0, 5}, {0, 1}, {1, 0}, {0, 0, 5}}, false},
		{"fewer than 3 unique points", [][]float64{{0, 0}, {0, 1}, {0, 1}, {0, 0}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCoordinates(tt.coordinates)
			if tt.wantValid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
			if !tt.wantValid && err == nil {
				t.Error("expected an error, got none")
			}
		})
	}
}

func TestParseID(t *testing.T) {
	if id, err := parseID("veh", "veh_456"); err != nil || id != 456 {
		t.Errorf(`parseID("veh", "veh_456") = %d, %v; want 456, nil`, id, err)
	}
	for _, bad := range []string{"", "456", "geo_456", "veh_", "veh_abc", "veh_-1", "veh_0"} {
		if _, err := parseID("veh", bad); err == nil {
			t.Errorf("parseID(%q) should fail", bad)
		}
	}
}

func TestFormatID(t *testing.T) {
	if got := formatID("geo", 12); got != "geo_12" {
		t.Errorf(`formatID("geo", 12) = %q, want "geo_12"`, got)
	}
}

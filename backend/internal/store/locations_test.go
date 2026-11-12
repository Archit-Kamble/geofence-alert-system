package store

import (
	"reflect"
	"testing"
)

func ids(refs []GeofenceRef) []int64 {
	out := []int64{}
	for _, g := range refs {
		out = append(out, g.ID)
	}
	return out
}

func TestDiffGeofences(t *testing.T) {
	a := GeofenceRef{ID: 1, Name: "A"}
	b := GeofenceRef{ID: 2, Name: "B"}
	c := GeofenceRef{ID: 3, Name: "C"}

	tests := []struct {
		name        string
		previous    []GeofenceRef
		current     []GeofenceRef
		wantEntered []int64
		wantExited  []int64
	}{
		{"first update inside one geofence", nil, []GeofenceRef{a}, []int64{1}, []int64{}},
		{"first update outside everything", nil, nil, []int64{}, []int64{}},
		{"stays inside: no event", []GeofenceRef{a}, []GeofenceRef{a}, []int64{}, []int64{}},
		{"leaves the geofence", []GeofenceRef{a}, nil, []int64{}, []int64{1}},
		{"moves from one geofence to another", []GeofenceRef{a}, []GeofenceRef{b}, []int64{2}, []int64{1}},
		{"overlapping geofences", []GeofenceRef{a, b}, []GeofenceRef{b, c}, []int64{3}, []int64{1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entered, exited := DiffGeofences(tt.previous, tt.current)
			if got := ids(entered); !reflect.DeepEqual(got, tt.wantEntered) {
				t.Errorf("entered = %v, want %v", got, tt.wantEntered)
			}
			if got := ids(exited); !reflect.DeepEqual(got, tt.wantExited) {
				t.Errorf("exited = %v, want %v", got, tt.wantExited)
			}
		})
	}
}

func TestPolygonWKTPutsLongitudeFirst(t *testing.T) {
	// The API uses [latitude, longitude]; PostGIS wants "longitude latitude".
	got := polygonWKT([][]float64{{12.5, 77.25}, {13, 77.25}, {13, 78}, {12.5, 77.25}})
	want := "POLYGON((77.25 12.5, 77.25 13, 78 13, 77.25 12.5))"
	if got != want {
		t.Errorf("polygonWKT = %q, want %q", got, want)
	}
}

import { memo, useEffect, useMemo } from "react";
import { CircleMarker, MapContainer, Polygon, Polyline, TileLayer, Tooltip, useMap, useMapEvents } from "react-leaflet";
import { categoryColor, categoryLabel } from "../constants.js";

const INDIA_CENTER = [20.5937, 78.9629];

// Leaflet uses [latitude, longitude], the same order as our API,
// so coordinates are passed to the map without swapping.

// ClickHandler reports map clicks as (latitude, longitude), rounded to 6 decimals (about 10 cm).
function ClickHandler({ onMapClick }) {
  useMapEvents({
    click(event) {
      const round = (value) => Math.round(value * 1e6) / 1e6;
      onMapClick(round(event.latlng.lat), round(event.latlng.lng));
    },
  });
  return null;
}

// ZoomTo moves the map so that the given points are visible.
function ZoomTo({ points }) {
  const map = useMap();
  useEffect(() => {
    if (points && points.length > 0) {
      map.fitBounds(points, { padding: [40, 40], maxZoom: 16 });
    }
  }, [map, points]);
  return null;
}

function MapView({ geofences, vehicles, locations, draft, selectedVehicleId, focusPoints, clickable, onMapClick }) {
  // Vehicles that have sent at least one location.
  const markers = useMemo(
    () =>
      vehicles
        .map((vehicle) => ({ vehicle, status: locations[vehicle.id] }))
        .filter((item) => item.status && item.status.current_location),
    [vehicles, locations]
  );

  return (
    <MapContainer center={INDIA_CENTER} zoom={5} className={clickable ? "map map-clickable" : "map"}>
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      />
      <ClickHandler onMapClick={onMapClick} />
      <ZoomTo points={focusPoints} />

      {geofences.map((geofence) => (
        <Polygon
          key={geofence.id}
          positions={geofence.coordinates}
          pathOptions={{ color: categoryColor(geofence.category), weight: 2, fillOpacity: 0.15 }}
        >
          <Tooltip sticky>
            <strong>{geofence.name}</strong>
            <br />
            {categoryLabel(geofence.category)}
          </Tooltip>
        </Polygon>
      ))}

      {/* The geofence being drawn: a dashed line through the clicked points. */}
      {draft.length > 0 && (
        <>
          <Polyline positions={[...draft, draft[0]]} pathOptions={{ color: "#111827", dashArray: "6 6", weight: 2 }} />
          {draft.map((point, index) => (
            <CircleMarker
              key={index}
              center={point}
              radius={5}
              pathOptions={{ color: "#111827", fillColor: "#ffffff", fillOpacity: 1, weight: 2 }}
            />
          ))}
        </>
      )}

      {markers.map(({ vehicle, status }) => {
        const selected = vehicle.id === selectedVehicleId;
        const inside = status.current_geofences.map((g) => g.geofence_name).join(", ");
        return (
          <CircleMarker
            key={vehicle.id}
            center={[status.current_location.latitude, status.current_location.longitude]}
            radius={selected ? 10 : 8}
            pathOptions={{
              color: "#ffffff",
              weight: 2,
              fillColor: selected ? "#7c3aed" : "#111827",
              fillOpacity: 1,
            }}
          >
            <Tooltip permanent direction="top" offset={[0, -8]}>
              {vehicle.vehicle_number}
              {inside && <span className="tooltip-inside"> · in {inside}</span>}
            </Tooltip>
          </CircleMarker>
        );
      })}
    </MapContainer>
  );
}

// memo: the map re-renders only when its own props change, not on every
// keystroke in a form elsewhere on the page.
export default memo(MapView);

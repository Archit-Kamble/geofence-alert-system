import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "./api.js";
import { useAlerts } from "./useAlerts.js";
import MapView from "./components/MapView.jsx";
import GeofencePanel from "./components/GeofencePanel.jsx";
import VehiclePanel from "./components/VehiclePanel.jsx";
import AlertRulesPanel from "./components/AlertRulesPanel.jsx";
import HistoryPanel from "./components/HistoryPanel.jsx";
import AlertFeed from "./components/AlertFeed.jsx";
import Toasts from "./components/Toasts.jsx";

const TABS = [
  { id: "geofences", label: "Geofences" },
  { id: "vehicles", label: "Vehicles" },
  { id: "rules", label: "Alert rules" },
  { id: "history", label: "History" },
];

const MAX_FEED_ALERTS = 50; // older live alerts are dropped from the feed
const TOAST_MS = 6000;

// App owns the data that more than one part of the screen needs:
// geofences and vehicles (map + panels), live alerts (feed + toasts),
// and what a click on the map currently means (mapMode).
export default function App() {
  const [geofences, setGeofences] = useState([]);
  const [vehicles, setVehicles] = useState([]);
  const [locations, setLocations] = useState({}); // vehicle id -> answer of GET /vehicles/location/{id}
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");

  const [tab, setTab] = useState("geofences");
  const [mapMode, setMapMode] = useState("idle"); // "idle" | "draw" | "move"
  const [draft, setDraft] = useState([]); // points of the geofence being drawn: [[lat, lng], ...]
  const [selectedVehicleId, setSelectedVehicleId] = useState("");
  const [movingVehicle, setMovingVehicle] = useState(false);
  const [moveMessage, setMoveMessage] = useState(null); // { type: "ok" | "error", text }
  const [focusPoints, setFocusPoints] = useState(null); // points the map should zoom to

  const [alerts, setAlerts] = useState([]);
  const [toasts, setToasts] = useState([]);
  const [historyVersion, setHistoryVersion] = useState(0); // changes when history should reload

  // ---- loading data -------------------------------------------------------

  const loadGeofences = useCallback(async () => {
    const data = await api.listGeofences();
    setGeofences(data.geofences);
    return data.geofences;
  }, []);

  const refreshLocation = useCallback(async (vehicleId) => {
    try {
      const location = await api.getLocation(vehicleId);
      setLocations((previous) => ({ ...previous, [vehicleId]: location }));
    } catch {
      // The marker keeps its old position; the next update will try again.
    }
  }, []);

  const loadVehicles = useCallback(async () => {
    const data = await api.listVehicles();
    setVehicles(data.vehicles);
    // The list endpoint has no positions, so ask for each vehicle's location.
    await Promise.all(data.vehicles.map((vehicle) => refreshLocation(vehicle.id)));
  }, [refreshLocation]);

  const loadEverything = useCallback(async () => {
    setLoading(true);
    setLoadError("");
    try {
      const [loadedGeofences] = await Promise.all([loadGeofences(), loadVehicles()]);
      // On first load, zoom the map to the geofences that already exist.
      const points = loadedGeofences.flatMap((geofence) => geofence.coordinates);
      if (points.length > 0) setFocusPoints(points);
    } catch (error) {
      setLoadError(error.message);
    } finally {
      setLoading(false);
    }
  }, [loadGeofences, loadVehicles]);

  useEffect(() => {
    loadEverything();
  }, [loadEverything]);

  // ---- live alerts ---------------------------------------------------------

  const toastTimers = useRef([]);
  useEffect(() => {
    const timers = toastTimers.current;
    return () => timers.forEach(clearTimeout);
  }, []);

  const dismissToast = useCallback((eventId) => {
    setToasts((previous) => previous.filter((toast) => toast.event_id !== eventId));
  }, []);

  const handleAlert = useCallback(
    (alert) => {
      setAlerts((previous) => [alert, ...previous].slice(0, MAX_FEED_ALERTS));
      setToasts((previous) => [...previous, alert]);
      toastTimers.current.push(setTimeout(() => dismissToast(alert.event_id), TOAST_MS));

      // Another browser tab may have moved the vehicle: fetch its new position.
      refreshLocation(alert.vehicle.vehicle_id);
      setHistoryVersion((version) => version + 1);
    },
    [dismissToast, refreshLocation]
  );

  const connectionStatus = useAlerts(handleAlert);

  // ---- actions ---------------------------------------------------------------

  // moveVehicle sends one location update and refreshes that vehicle's status.
  const moveVehicle = useCallback(
    async (vehicleId, latitude, longitude, timestamp) => {
      setMovingVehicle(true);
      setMoveMessage(null);
      try {
        const result = await api.updateLocation({ vehicle_id: vehicleId, latitude, longitude, timestamp });
        const names = result.current_geofences.map((g) => g.geofence_name);
        setMoveMessage({
          type: "ok",
          text: names.length ? `Location updated. Inside: ${names.join(", ")}.` : "Location updated. Not inside any geofence.",
        });
        await refreshLocation(vehicleId);
        setHistoryVersion((version) => version + 1);
      } catch (error) {
        setMoveMessage({ type: "error", text: error.message });
      } finally {
        setMovingVehicle(false);
      }
    },
    [refreshLocation]
  );

  // What a click on the map does depends on the current mode.
  const handleMapClick = useCallback(
    (latitude, longitude) => {
      if (mapMode === "draw") {
        setDraft((points) => [...points, [latitude, longitude]]);
      } else if (mapMode === "move" && selectedVehicleId && !movingVehicle) {
        moveVehicle(selectedVehicleId, latitude, longitude, new Date().toISOString());
      }
    },
    [mapMode, selectedVehicleId, movingVehicle, moveVehicle]
  );

  const focusGeofence = useCallback((geofence) => setFocusPoints(geofence.coordinates), []);
  const clearAlerts = useCallback(() => setAlerts([]), []);

  const selectTab = (id) => {
    setTab(id);
    setMapMode("idle"); // leaving a tab cancels drawing or moving
  };

  // ---- screen ---------------------------------------------------------------

  return (
    <div className="app">
      <header className="topbar">
        <h1>Geofence Alerts</h1>
        <span className={`status status-${connectionStatus}`}>
          {connectionStatus === "open" && "Live alerts connected"}
          {connectionStatus === "connecting" && "Connecting to live alerts…"}
          {connectionStatus === "closed" && "Live alerts disconnected. Retrying…"}
        </span>
      </header>

      <div className="layout">
        <aside className="sidebar">
          <nav className="tabs">
            {TABS.map((item) => (
              <button
                key={item.id}
                className={item.id === tab ? "tab tab-active" : "tab"}
                onClick={() => selectTab(item.id)}
              >
                {item.label}
              </button>
            ))}
          </nav>

          <div className="panel">
            {loadError && (
              <div className="message message-error">
                {loadError}{" "}
                <button className="link" onClick={loadEverything}>
                  Try again
                </button>
              </div>
            )}

            {tab === "geofences" && (
              <GeofencePanel
                geofences={geofences}
                loading={loading}
                draft={draft}
                setDraft={setDraft}
                drawing={mapMode === "draw"}
                setDrawing={(on) => setMapMode(on ? "draw" : "idle")}
                onCreated={loadGeofences}
                onFocus={focusGeofence}
              />
            )}
            {tab === "vehicles" && (
              <VehiclePanel
                vehicles={vehicles}
                locations={locations}
                loading={loading}
                selectedVehicleId={selectedVehicleId}
                setSelectedVehicleId={setSelectedVehicleId}
                movingOnMap={mapMode === "move"}
                setMovingOnMap={(on) => setMapMode(on ? "move" : "idle")}
                moveVehicle={moveVehicle}
                moving={movingVehicle}
                moveMessage={moveMessage}
                onCreated={loadVehicles}
              />
            )}
            {tab === "rules" && <AlertRulesPanel geofences={geofences} vehicles={vehicles} />}
            {tab === "history" && (
              <HistoryPanel geofences={geofences} vehicles={vehicles} version={historyVersion} />
            )}
          </div>
        </aside>

        <main className="map-area">
          {mapMode === "draw" && <div className="map-hint">Click the map to add corner points of the geofence.</div>}
          {mapMode === "move" && (
            <div className="map-hint">
              {selectedVehicleId ? "Click the map to move the selected vehicle there." : "Select a vehicle first."}
            </div>
          )}
          <MapView
            geofences={geofences}
            vehicles={vehicles}
            locations={locations}
            draft={draft}
            selectedVehicleId={selectedVehicleId}
            focusPoints={focusPoints}
            clickable={mapMode !== "idle"}
            onMapClick={handleMapClick}
          />
        </main>

        <AlertFeed alerts={alerts} onClear={clearAlerts} />
      </div>

      <Toasts toasts={toasts} onDismiss={dismissToast} />
    </div>
  );
}

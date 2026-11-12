import { useState } from "react";
import { api } from "../api.js";
import { formatTime, localInputValue } from "../constants.js";

const EMPTY_VEHICLE = { vehicle_number: "", driver_name: "", vehicle_type: "truck", phone: "" };
const VEHICLE_TYPES = ["truck", "car", "van", "motorcycle"];

export default function VehiclePanel({
  vehicles,
  locations,
  loading,
  selectedVehicleId,
  setSelectedVehicleId,
  movingOnMap,
  setMovingOnMap,
  moveVehicle,
  moving,
  moveMessage,
  onCreated,
}) {
  // ---- register form ----
  const [form, setForm] = useState(EMPTY_VEHICLE);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState(null);

  // ---- manual location form ----
  const [latitude, setLatitude] = useState("");
  const [longitude, setLongitude] = useState("");
  const [timestamp, setTimestamp] = useState(localInputValue());

  const setField = (field) => (event) => setForm({ ...form, [field]: event.target.value });

  async function handleRegister(event) {
    event.preventDefault();
    setSaving(true);
    setMessage(null);
    try {
      const created = await api.createVehicle(form);
      setMessage({ type: "ok", text: `Vehicle ${created.vehicle_number} registered.` });
      setForm(EMPTY_VEHICLE);
      await onCreated();
      setSelectedVehicleId(created.id);
    } catch (error) {
      setMessage({ type: "error", text: error.message });
    } finally {
      setSaving(false);
    }
  }

  function handleManualUpdate(event) {
    event.preventDefault();
    // The datetime-local input gives local time; the API wants ISO 8601 (UTC).
    moveVehicle(selectedVehicleId, Number(latitude), Number(longitude), new Date(timestamp).toISOString());
  }

  return (
    <>
      <h2>Register vehicle</h2>
      <form className="form" onSubmit={handleRegister}>
        <label>
          Vehicle number
          <input value={form.vehicle_number} onChange={setField("vehicle_number")} required placeholder="KA-01-AB-1234" />
        </label>
        <label>
          Driver name
          <input value={form.driver_name} onChange={setField("driver_name")} required />
        </label>
        <div className="form-row">
          <label>
            Type
            <select value={form.vehicle_type} onChange={setField("vehicle_type")}>
              {VEHICLE_TYPES.map((type) => (
                <option key={type}>{type}</option>
              ))}
            </select>
          </label>
          <label>
            Phone
            <input value={form.phone} onChange={setField("phone")} required placeholder="+91…" />
          </label>
        </div>
        {message && <div className={`message message-${message.type}`}>{message.text}</div>}
        <button className="button button-primary" disabled={saving}>
          {saving ? "Saving…" : "Register vehicle"}
        </button>
      </form>

      <h2>Update location</h2>
      <div className="form">
        <label>
          Vehicle
          <select value={selectedVehicleId} onChange={(e) => setSelectedVehicleId(e.target.value)}>
            <option value="">Select a vehicle…</option>
            {vehicles.map((vehicle) => (
              <option key={vehicle.id} value={vehicle.id}>
                {vehicle.vehicle_number} ({vehicle.driver_name})
              </option>
            ))}
          </select>
        </label>

        <button
          type="button"
          className={movingOnMap ? "button button-active" : "button"}
          onClick={() => setMovingOnMap(!movingOnMap)}
          disabled={!selectedVehicleId}
        >
          {movingOnMap ? "Stop moving on map" : "Move by clicking the map"}
        </button>

        <details>
          <summary>Or type a position</summary>
          <form className="form" onSubmit={handleManualUpdate}>
            <div className="form-row">
              <label>
                Latitude
                <input type="number" step="any" min="-90" max="90" value={latitude} onChange={(e) => setLatitude(e.target.value)} required />
              </label>
              <label>
                Longitude
                <input type="number" step="any" min="-180" max="180" value={longitude} onChange={(e) => setLongitude(e.target.value)} required />
              </label>
            </div>
            <label>
              Time recorded
              <input type="datetime-local" value={timestamp} onChange={(e) => setTimestamp(e.target.value)} required />
            </label>
            <button className="button" disabled={!selectedVehicleId || moving}>
              {moving ? "Updating…" : "Update location"}
            </button>
          </form>
        </details>

        {moving && <p className="muted">Updating location…</p>}
        {moveMessage && <div className={`message message-${moveMessage.type}`}>{moveMessage.text}</div>}
      </div>

      <h2>Vehicles ({vehicles.length})</h2>
      {loading ? (
        <p className="muted">Loading…</p>
      ) : vehicles.length === 0 ? (
        <p className="muted">No vehicles yet. Register the first one above.</p>
      ) : (
        <ul className="list">
          {vehicles.map((vehicle) => {
            const status = locations[vehicle.id];
            const position = status && status.current_location;
            const inside = status ? status.current_geofences : [];
            return (
              <li key={vehicle.id} className={vehicle.id === selectedVehicleId ? "list-item list-item-selected" : "list-item"}>
                <div className="list-text">
                  <strong>{vehicle.vehicle_number}</strong>
                  <span className="muted">
                    {vehicle.driver_name} · {vehicle.vehicle_type} · {vehicle.phone}
                  </span>
                  {position ? (
                    <span className="muted">
                      {position.latitude.toFixed(5)}, {position.longitude.toFixed(5)} at {formatTime(position.timestamp)}
                    </span>
                  ) : (
                    <span className="muted">No location sent yet</span>
                  )}
                  <span>
                    {inside.length > 0 ? (
                      inside.map((g) => (
                        <span key={g.geofence_id} className="badge">
                          in {g.geofence_name}
                        </span>
                      ))
                    ) : (
                      position && <span className="muted">Outside all geofences</span>
                    )}
                  </span>
                </div>
                <button className="button button-small" onClick={() => setSelectedVehicleId(vehicle.id)}>
                  Select
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </>
  );
}

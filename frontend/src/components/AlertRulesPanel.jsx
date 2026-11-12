import { useCallback, useEffect, useState } from "react";
import { api } from "../api.js";

const EVENT_TYPES = [
  { value: "entry", label: "Entry" },
  { value: "exit", label: "Exit" },
  { value: "both", label: "Entry and exit" },
];

export default function AlertRulesPanel({ geofences, vehicles }) {
  const [rules, setRules] = useState([]);
  const [loading, setLoading] = useState(true);
  const [geofenceId, setGeofenceId] = useState("");
  const [vehicleId, setVehicleId] = useState(""); // empty means all vehicles
  const [eventType, setEventType] = useState("entry");
  const [saving, setSaving] = useState(false);
  const [deletingId, setDeletingId] = useState("");
  const [message, setMessage] = useState(null);

  const loadRules = useCallback(async () => {
    try {
      const data = await api.listAlertRules();
      setRules(data.alerts);
    } catch (error) {
      setMessage({ type: "error", text: error.message });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadRules();
  }, [loadRules]);

  async function handleSubmit(event) {
    event.preventDefault();
    setSaving(true);
    setMessage(null);
    try {
      const rule = { geofence_id: geofenceId, event_type: eventType };
      if (vehicleId) rule.vehicle_id = vehicleId; // left out: the rule applies to all vehicles
      await api.configureAlert(rule);
      setMessage({ type: "ok", text: "Alert rule saved." });
      await loadRules();
    } catch (error) {
      setMessage({ type: "error", text: error.message });
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(alertId) {
    setDeletingId(alertId);
    setMessage(null);
    try {
      await api.deleteAlertRule(alertId);
      await loadRules();
    } catch (error) {
      setMessage({ type: "error", text: error.message });
    } finally {
      setDeletingId("");
    }
  }

  return (
    <>
      <h2>New alert rule</h2>
      <form className="form" onSubmit={handleSubmit}>
        <label>
          Geofence
          <select value={geofenceId} onChange={(e) => setGeofenceId(e.target.value)} required>
            <option value="">Select a geofence…</option>
            {geofences.map((geofence) => (
              <option key={geofence.id} value={geofence.id}>
                {geofence.name}
              </option>
            ))}
          </select>
        </label>
        <label>
          Vehicle
          <select value={vehicleId} onChange={(e) => setVehicleId(e.target.value)}>
            <option value="">All vehicles</option>
            {vehicles.map((vehicle) => (
              <option key={vehicle.id} value={vehicle.id}>
                {vehicle.vehicle_number}
              </option>
            ))}
          </select>
        </label>
        <label>
          Alert on
          <select value={eventType} onChange={(e) => setEventType(e.target.value)}>
            {EVENT_TYPES.map((type) => (
              <option key={type.value} value={type.value}>
                {type.label}
              </option>
            ))}
          </select>
        </label>
        {message && <div className={`message message-${message.type}`}>{message.text}</div>}
        <button className="button button-primary" disabled={saving}>
          {saving ? "Saving…" : "Save rule"}
        </button>
      </form>

      <p className="muted note">Entering a restricted zone always raises an alert, even without a rule.</p>

      <h2>Alert rules ({rules.length})</h2>
      {loading ? (
        <p className="muted">Loading…</p>
      ) : rules.length === 0 ? (
        <p className="muted">No rules yet.</p>
      ) : (
        <ul className="list">
          {rules.map((rule) => (
            <li key={rule.alert_id} className="list-item">
              <div className="list-text">
                <strong>{rule.geofence_name}</strong>
                <span className="muted">
                  {rule.vehicle_number || "All vehicles"} · {EVENT_TYPES.find((t) => t.value === rule.event_type)?.label}
                </span>
              </div>
              <button
                className="button button-small button-danger"
                onClick={() => handleDelete(rule.alert_id)}
                disabled={deletingId === rule.alert_id}
              >
                {deletingId === rule.alert_id ? "Removing…" : "Remove"}
              </button>
            </li>
          ))}
        </ul>
      )}
    </>
  );
}

import { useEffect, useState } from "react";
import { api } from "../api.js";
import { CATEGORIES, categoryColor, categoryLabel } from "../constants.js";

// parseCoordinates reads "latitude, longitude" lines typed by the user.
// It returns { points } or { error }.
function parseCoordinates(text) {
  const points = [];
  const lines = text.split("\n").map((line) => line.trim()).filter(Boolean);
  for (const [index, line] of lines.entries()) {
    const parts = line.split(/[,\s]+/).filter(Boolean).map(Number);
    if (parts.length !== 2 || parts.some(Number.isNaN)) {
      return { error: `Line ${index + 1} must be "latitude, longitude".` };
    }
    points.push(parts);
  }
  return { points };
}

export default function GeofencePanel({ geofences, loading, draft, setDraft, drawing, setDrawing, onCreated, onFocus }) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [category, setCategory] = useState(CATEGORIES[0].value);
  const [coordinateText, setCoordinateText] = useState("");
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState(null);

  // The list below the form. With a filter chosen it is fetched from
  // GET /geofences?category=..., otherwise it is the full list from App.
  const [filter, setFilter] = useState("");
  const [filtered, setFiltered] = useState(null);
  const [filterLoading, setFilterLoading] = useState(false);

  useEffect(() => {
    if (!filter) {
      setFiltered(null);
      return;
    }
    let cancelled = false; // ignore an answer that arrives after the filter changed again
    setFilterLoading(true);
    api
      .listGeofences(filter)
      .then((data) => !cancelled && setFiltered(data.geofences))
      .catch((error) => !cancelled && setMessage({ type: "error", text: error.message }))
      .finally(() => !cancelled && setFilterLoading(false));
    return () => {
      cancelled = true;
    };
  }, [filter, geofences]);

  const visible = filtered || geofences;

  function applyTypedCoordinates() {
    const result = parseCoordinates(coordinateText);
    if (result.error) {
      setMessage({ type: "error", text: result.error });
      return;
    }
    // If the user already closed the polygon, drop the repeated last point:
    // the closing point is added once, when saving.
    const points = result.points;
    const last = points[points.length - 1];
    if (points.length > 1 && last[0] === points[0][0] && last[1] === points[0][1]) {
      points.pop();
    }
    setDraft(points);
    setMessage(null);
  }

  async function handleSubmit(event) {
    event.preventDefault();
    if (draft.length < 3) {
      setMessage({ type: "error", text: "A geofence needs at least 3 points. Draw them on the map or type them below." });
      return;
    }

    setSaving(true);
    setMessage(null);
    try {
      // The backend wants a closed polygon: the first point repeated at the end.
      const coordinates = [...draft, draft[0]];
      const created = await api.createGeofence({ name, description, category, coordinates });
      setMessage({ type: "ok", text: `Geofence "${created.name}" created.` });
      setName("");
      setDescription("");
      setCoordinateText("");
      setDraft([]);
      setDrawing(false);
      await onCreated();
    } catch (error) {
      setMessage({ type: "error", text: error.message });
    } finally {
      setSaving(false);
    }
  }

  return (
    <>
      <h2>New geofence</h2>
      <form className="form" onSubmit={handleSubmit}>
        <label>
          Name
          <input value={name} onChange={(e) => setName(e.target.value)} required placeholder="Downtown Delivery Zone" />
        </label>
        <label>
          Description (optional)
          <input value={description} onChange={(e) => setDescription(e.target.value)} />
        </label>
        <label>
          Category
          <select value={category} onChange={(e) => setCategory(e.target.value)}>
            {CATEGORIES.map((c) => (
              <option key={c.value} value={c.value}>
                {c.label}
              </option>
            ))}
          </select>
        </label>

        <div className="field">
          <span className="field-label">Boundary: {draft.length} point{draft.length === 1 ? "" : "s"}</span>
          <div className="button-row">
            <button type="button" className={drawing ? "button button-active" : "button"} onClick={() => setDrawing(!drawing)}>
              {drawing ? "Stop drawing" : "Draw on map"}
            </button>
            <button type="button" className="button" onClick={() => setDraft(draft.slice(0, -1))} disabled={draft.length === 0}>
              Undo point
            </button>
            <button type="button" className="button" onClick={() => setDraft([])} disabled={draft.length === 0}>
              Clear
            </button>
          </div>
        </div>

        <details>
          <summary>Or type coordinates</summary>
          <textarea
            rows={4}
            value={coordinateText}
            onChange={(e) => setCoordinateText(e.target.value)}
            placeholder={"One point per line: latitude, longitude\n12.97, 77.59\n12.98, 77.59\n12.98, 77.60"}
          />
          <button type="button" className="button" onClick={applyTypedCoordinates} disabled={!coordinateText.trim()}>
            Show these points on the map
          </button>
        </details>

        {message && <div className={`message message-${message.type}`}>{message.text}</div>}

        <button className="button button-primary" disabled={saving}>
          {saving ? "Saving…" : "Create geofence"}
        </button>
      </form>

      <div className="list-header">
        <h2>Geofences ({visible.length})</h2>
        <select value={filter} onChange={(e) => setFilter(e.target.value)} aria-label="Filter by category">
          <option value="">All categories</option>
          {CATEGORIES.map((c) => (
            <option key={c.value} value={c.value}>
              {c.label}
            </option>
          ))}
        </select>
      </div>

      {loading || filterLoading ? (
        <p className="muted">Loading…</p>
      ) : visible.length === 0 ? (
        <p className="muted">{filter ? "No geofences in this category." : "No geofences yet. Create the first one above."}</p>
      ) : (
        <ul className="list">
          {visible.map((geofence) => (
            <li key={geofence.id} className="list-item">
              <span className="dot" style={{ background: categoryColor(geofence.category) }} />
              <div className="list-text">
                <strong>{geofence.name}</strong>
                <span className="muted">
                  {categoryLabel(geofence.category)} · {geofence.id}
                </span>
                {geofence.description && <span className="muted">{geofence.description}</span>}
              </div>
              <button className="button button-small" onClick={() => onFocus(geofence)}>
                Show
              </button>
            </li>
          ))}
        </ul>
      )}
    </>
  );
}

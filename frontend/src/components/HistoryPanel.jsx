import { useEffect, useState } from "react";
import { api } from "../api.js";
import { formatTime } from "../constants.js";

const PAGE_SIZE = 10;

export default function HistoryPanel({ geofences, vehicles, version }) {
  const [vehicleId, setVehicleId] = useState("");
  const [geofenceId, setGeofenceId] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [page, setPage] = useState(0);

  const [events, setEvents] = useState([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  // Changing a filter goes back to the first page.
  const changeFilter = (setter) => (event) => {
    setter(event.target.value);
    setPage(0);
  };

  // Load a page whenever a filter, the page, or "version" changes.
  // "version" changes when a new event happened, so the table stays fresh.
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError("");
    api
      .history({
        vehicle_id: vehicleId,
        geofence_id: geofenceId,
        // datetime-local gives local time; the API wants ISO 8601.
        start_date: startDate ? new Date(startDate).toISOString() : "",
        end_date: endDate ? new Date(endDate).toISOString() : "",
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
      })
      .then((data) => {
        if (cancelled) return;
        setEvents(data.violations);
        setTotal(data.total_count);
      })
      .catch((err) => !cancelled && setError(err.message))
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [vehicleId, geofenceId, startDate, endDate, page, version]);

  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const hasFilters = vehicleId || geofenceId || startDate || endDate;

  function clearFilters() {
    setVehicleId("");
    setGeofenceId("");
    setStartDate("");
    setEndDate("");
    setPage(0);
  }

  return (
    <>
      <h2>Entry and exit history</h2>
      <div className="form">
        <div className="form-row">
          <label>
            Vehicle
            <select value={vehicleId} onChange={changeFilter(setVehicleId)}>
              <option value="">All vehicles</option>
              {vehicles.map((vehicle) => (
                <option key={vehicle.id} value={vehicle.id}>
                  {vehicle.vehicle_number}
                </option>
              ))}
            </select>
          </label>
          <label>
            Geofence
            <select value={geofenceId} onChange={changeFilter(setGeofenceId)}>
              <option value="">All geofences</option>
              {geofences.map((geofence) => (
                <option key={geofence.id} value={geofence.id}>
                  {geofence.name}
                </option>
              ))}
            </select>
          </label>
        </div>
        <div className="form-row">
          <label>
            From
            <input type="datetime-local" value={startDate} onChange={changeFilter(setStartDate)} />
          </label>
          <label>
            To
            <input type="datetime-local" value={endDate} onChange={changeFilter(setEndDate)} />
          </label>
        </div>
        {hasFilters && (
          <button type="button" className="button" onClick={clearFilters}>
            Clear filters
          </button>
        )}
      </div>

      {error && <div className="message message-error">{error}</div>}

      {loading && events.length === 0 ? (
        <p className="muted">Loading…</p>
      ) : events.length === 0 ? (
        <p className="muted">{hasFilters ? "No events match these filters." : "No events yet. Move a vehicle into a geofence."}</p>
      ) : (
        <ul className={loading ? "list list-loading" : "list"}>
          {events.map((event) => (
            <li key={event.id} className="list-item">
              <span className={`pill pill-${event.event_type}`}>{event.event_type}</span>
              <div className="list-text">
                <strong>
                  {event.vehicle_number} · {event.geofence_name}
                </strong>
                <span className="muted">
                  {formatTime(event.timestamp)} · {event.latitude.toFixed(5)}, {event.longitude.toFixed(5)}
                </span>
              </div>
            </li>
          ))}
        </ul>
      )}

      <div className="pager">
        <button className="button button-small" onClick={() => setPage(page - 1)} disabled={page === 0 || loading}>
          Previous
        </button>
        <span className="muted">
          Page {page + 1} of {pageCount} · {total} event{total === 1 ? "" : "s"}
        </span>
        <button className="button button-small" onClick={() => setPage(page + 1)} disabled={page + 1 >= pageCount || loading}>
          Next
        </button>
      </div>
    </>
  );
}

import { memo } from "react";
import { categoryLabel, formatTime } from "../constants.js";

// One alert in the feed. memo: when a new alert arrives, only the new row is
// rendered; the rows already on screen are skipped because their props did not change.
const AlertRow = memo(function AlertRow({ alert }) {
  return (
    <li className={`alert alert-${alert.event_type}`}>
      <div className="alert-top">
        <span className={`pill pill-${alert.event_type}`}>{alert.event_type}</span>
        <span className="muted">{formatTime(alert.timestamp)}</span>
      </div>
      <strong>{alert.vehicle.vehicle_number}</strong>
      <span>
        {alert.event_type === "entry" ? "entered" : "left"} {alert.geofence.geofence_name}
      </span>
      <span className="muted">
        {categoryLabel(alert.geofence.category)} · driver {alert.vehicle.driver_name}
      </span>
      <span className="muted">
        {alert.location.latitude.toFixed(5)}, {alert.location.longitude.toFixed(5)}
      </span>
    </li>
  );
});

function AlertFeed({ alerts, onClear }) {
  return (
    <aside className="feed">
      <div className="list-header">
        <h2>Live alerts ({alerts.length})</h2>
        {alerts.length > 0 && (
          <button className="button button-small" onClick={onClear}>
            Clear
          </button>
        )}
      </div>
      {alerts.length === 0 ? (
        <p className="muted">Alerts appear here the moment a vehicle enters or leaves a watched geofence.</p>
      ) : (
        <ul className="alert-list">
          {alerts.map((alert) => (
            <AlertRow key={alert.event_id} alert={alert} />
          ))}
        </ul>
      )}
    </aside>
  );
}

export default memo(AlertFeed);

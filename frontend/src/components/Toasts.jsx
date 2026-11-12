import { memo } from "react";

// Toasts are the short pop-up notices in the corner. App removes each one after a few seconds.
function Toasts({ toasts, onDismiss }) {
  return (
    <div className="toasts" role="status" aria-live="polite">
      {toasts.map((alert) => (
        <div key={alert.event_id} className={`toast toast-${alert.event_type}`}>
          <div>
            <strong>{alert.vehicle.vehicle_number}</strong> {alert.event_type === "entry" ? "entered" : "left"}{" "}
            <strong>{alert.geofence.geofence_name}</strong>
          </div>
          <button className="toast-close" onClick={() => onDismiss(alert.event_id)} aria-label="Dismiss">
            ×
          </button>
        </div>
      ))}
    </div>
  );
}

export default memo(Toasts);

// Every call to the Go backend goes through this file.

// The backend address comes from an environment variable set at build time,
// so the same code works on a laptop and when deployed.
const API_URL = (import.meta.env.VITE_API_URL || "http://localhost:8080").replace(/\/$/, "");

// http://host -> ws://host and https://host -> wss://host
export const WS_URL = API_URL.replace(/^http/, "ws") + "/ws/alerts";

// request sends one HTTP call and returns the parsed JSON.
// When the backend answers with an error status, it throws an Error
// that carries the backend's own message, so the UI can show it.
async function request(path, options = {}) {
  let response;
  try {
    response = await fetch(API_URL + path, {
      headers: options.body ? { "Content-Type": "application/json" } : undefined,
      ...options,
    });
  } catch {
    throw new Error("Cannot reach the server. Check that the backend is running.");
  }

  let data = null;
  try {
    data = await response.json();
  } catch {
    // the body was not JSON; handled below
  }

  if (!response.ok) {
    throw new Error((data && data.error) || `Request failed (${response.status})`);
  }
  return data;
}

// query builds "?a=1&b=2" and leaves out empty values.
function query(params) {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== "" && value !== undefined && value !== null) {
      search.set(key, value);
    }
  }
  const text = search.toString();
  return text ? "?" + text : "";
}

const post = (path, body) => request(path, { method: "POST", body: JSON.stringify(body) });

export const api = {
  listGeofences: (category) => request("/geofences" + query({ category })),
  createGeofence: (geofence) => post("/geofences", geofence),

  listVehicles: () => request("/vehicles"),
  createVehicle: (vehicle) => post("/vehicles", vehicle),

  updateLocation: (update) => post("/vehicles/location", update),
  getLocation: (vehicleId) => request("/vehicles/location/" + encodeURIComponent(vehicleId)),

  listAlertRules: () => request("/alerts"),
  configureAlert: (rule) => post("/alerts/configure", rule),
  deleteAlertRule: (alertId) => request("/alerts/" + encodeURIComponent(alertId), { method: "DELETE" }),

  history: (filters) => request("/violations/history" + query(filters)),
};

# Geofencing & Real-time Alert System

Draw zones on a map, register vehicles, send their positions, and get a live alert the moment a vehicle enters or leaves a zone.

Submission for the MapUp Full-Stack Developer assessment.

| | |
| --- | --- |
| Video walkthrough | _TODO: add link_ |
| Frontend (live) | https://geofence-alert-system-three.vercel.app |
| Backend (live) | https://geofence-backend-d7k1.onrender.com ([health check](https://geofence-backend-d7k1.onrender.com/health)) |
| Docker Hub image | [architkamble/geofence-backend](https://hub.docker.com/r/architkamble/geofence-backend) (`docker pull architkamble/geofence-backend:latest`) |
| Setup and API guide | [SETUP.md](SETUP.md) |

## Stack

- **Backend:** Go 1.22 (standard library router), `pgx` for PostgreSQL, `gorilla/websocket`
- **Database:** PostgreSQL 16 + PostGIS 3
- **Frontend:** React 18 + Vite, Leaflet (react-leaflet), plain CSS
- **Infrastructure:** Docker (multi-stage builds), Docker Compose

## Quick start

```bash
docker compose up --build
```

Frontend: http://localhost:5173 · Backend: http://localhost:8080 · Details: [SETUP.md](SETUP.md)

## Project structure

```
backend/
  main.go                 settings, wiring, graceful shutdown
  internal/api/           HTTP layer: one file per resource (geofences, vehicles, locations, alerts, violations)
    respond.go            JSON responses with time_ns, id formatting
    validate.go           geofence coordinate rules
  internal/store/         database layer: all SQL
    schema.sql            tables and indexes, created at startup
    locations.go          location update, entry/exit detection
  internal/ws/hub.go      WebSocket hub: connected clients and broadcast
frontend/
  src/App.jsx             shared state and page layout
  src/api.js              all HTTP calls
  src/useAlerts.js        WebSocket hook with automatic reconnect
  src/components/         MapView, four tab panels, AlertFeed, Toasts
docker-compose.yml        database + backend + frontend
```

The backend has two layers. Handlers in `api` deal with HTTP and validation; `store` deals with SQL. A handler never writes SQL and the store never sees an HTTP request.

## API

All nine endpoints from the brief are implemented with the exact paths, methods and field names. Every response, including errors, has `time_ns`.

| Method | Path | Purpose |
| --- | --- | --- |
| POST | `/geofences` | Create a geofence |
| GET | `/geofences` | List geofences (`category` filter) |
| POST | `/vehicles` | Register a vehicle |
| GET | `/vehicles` | List vehicles |
| POST | `/vehicles/location` | Update a location, detect entry/exit, raise alerts |
| GET | `/vehicles/location/{vehicle_id}` | Current location and geofences |
| POST | `/alerts/configure` | Create an alert rule |
| GET | `/alerts` | List alert rules (`geofence_id`, `vehicle_id` filters) |
| GET | `/violations/history` | Entry/exit history with filters |
| WS | `/ws/alerts` | Live alerts |

Two additions that do not change the endpoints above:

- `DELETE /alerts/{alert_id}` so the UI can remove a rule.
- `offset` on `/violations/history`, for pagination together with `limit` and `total_count`.

## Design decisions

**Coordinate order.** The API uses `[latitude, longitude]` as the brief says. PostGIS wants longitude first. The swap happens in exactly two places in `internal/store` (`polygonWKT` and the `ST_MakePoint` call), and a unit test covers it.

**Point-in-polygon.** Done by PostGIS with `ST_Covers(boundary, point)`. A point exactly on the boundary counts as inside (`ST_Contains` would say outside). A GiST index on the polygon column lets the query skip geofences whose bounding box does not contain the point.

**Entry and exit are changes, not states.** The table `vehicle_geofence_state` holds the geofences each vehicle is inside now. On each update the new set is compared with the stored set: new ids are entries, missing ids are exits. A vehicle that stays inside produces no event. On a vehicle's first update the stored set is empty, so being inside a geofence counts as an entry.

**Concurrent updates for one vehicle.** The location update runs in one transaction that first locks the vehicle row (`SELECT ... FOR UPDATE`). Two updates for the same vehicle are processed one after the other, so the same entry cannot be recorded twice.

**Which events raise an alert.** Every entry and exit is stored in `geofence_events` and appears in the history. An alert is pushed when a configured rule matches (same geofence; that vehicle or all vehicles; that event type or `both`). Entering a `restricted_zone` always raises an alert, even without a rule.

**Alerts do not slow the API.** The handler only puts the alert on a buffered channel and returns. A separate goroutine reads the channel and writes to each WebSocket client. A slow or dead client is dropped instead of blocking the others. The client list is protected by a mutex, and the tests run under Go's race detector.

**Polygon validation.** The handler checks the rules in the brief (closed, at least 4 points, valid ranges, allowed category). PostGIS then rejects polygons whose boundary crosses itself (`ST_IsValid`).

**IDs.** Stored as numbers, shown with a prefix (`geo_1`, `veh_1`, `alert_1`, `viol_1`, `evt_1`).

## Known gaps and what I would do next

- **No authentication or rate limiting.** Next: API keys or JWT, and a per-vehicle rate limit on location updates.
- **Out-of-order location updates.** Updates are processed in arrival order. A late update with an older timestamp is treated as the newest position. Next: ignore updates older than the last one for state changes.
- **Geofences cannot be edited or deleted** (the brief has no endpoint for it).
- **A single backend instance.** The hub lives in memory, so with several instances a client would only get alerts raised on its own instance. Next: publish alerts through Redis pub/sub or PostgreSQL `LISTEN/NOTIFY`.
- **Alerts missed while offline are not replayed.** They are in the history, but the live feed starts empty after a reload. Next: send the last N alerts on connect.
- **The vehicle list needs one request per vehicle** for positions. Next: one endpoint that returns all current positions.
- **Tests** cover validation, id parsing, the entry/exit comparison and the WebSocket hub. There are no automated tests against a real database yet.
- **Scale.** For thousands of vehicles: keep each vehicle's current geofence set in Redis, batch location inserts, and partition the location and event tables by time.

## How AI was used

MapUp's instructions allow AI tools. This project was built with an AI coding assistant (Claude). I set the requirements from the brief, reviewed the design and the code, ran and tested it, and I can explain every part of it.

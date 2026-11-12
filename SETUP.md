# Setup Guide

## 1. Prerequisites

| To run with Docker (recommended) | To run without Docker |
| --- | --- |
| Docker 24+ with Docker Compose v2 | Go 1.22+ |
| | Node.js 20+ and npm |
| | PostgreSQL 16 with the PostGIS 3 extension |

## 2. Run with Docker Compose

From the repository root:

```bash
docker compose up --build
```

This starts three containers:

| Service | Address | What it is |
| --- | --- | --- |
| `frontend` | http://localhost:5173 | React app served by nginx |
| `backend` | http://localhost:8080 | Go API and WebSocket server |
| `db` | (internal only) | PostgreSQL 16 + PostGIS |

The backend creates its tables at startup, so there is no migration step.
Stop with `Ctrl+C`. To also delete the stored data: `docker compose down -v`.

## 3. Run locally without Docker

### Database

```bash
sudo -u postgres psql -c "CREATE ROLE geofence LOGIN PASSWORD 'geofence'"
sudo -u postgres psql -c "CREATE DATABASE geofence OWNER geofence"
sudo -u postgres psql -d geofence -c "CREATE EXTENSION IF NOT EXISTS postgis"
```

### Backend

```bash
cd backend
go run .
```

Settings are read from environment variables:

| Variable | Default | Meaning |
| --- | --- | --- |
| `DATABASE_URL` | `postgres://geofence:geofence@localhost:5432/geofence?sslmode=disable` | PostgreSQL connection string |
| `PORT` | `8080` | Port the API listens on |
| `ALLOWED_ORIGINS` | `*` | Comma-separated list of websites allowed to call the API (CORS and WebSocket) |

Run the tests (the `-race` flag turns on Go's race detector):

```bash
go test -race ./...
```

### Frontend

```bash
cd frontend
cp .env.example .env      # set VITE_API_URL if the backend is not on localhost:8080
npm install
npm run dev               # http://localhost:5173
```

## 4. API testing guide

Every response, including errors, carries `time_ns` (handler execution time in nanoseconds).
Coordinates are always `[latitude, longitude]`.

```bash
API=http://localhost:8080
```

**Create a geofence** (closed polygon: first point = last point, at least 4 points)

```bash
curl -s -X POST $API/geofences -H 'Content-Type: application/json' -d '{
  "name": "Downtown Delivery Zone",
  "description": "Main delivery area",
  "coordinates": [[12.90,77.50],[13.00,77.50],[13.00,77.60],[12.90,77.60],[12.90,77.50]],
  "category": "delivery_zone"
}'
# {"id":"geo_1","name":"Downtown Delivery Zone","status":"active","time_ns":"..."}
```

**List geofences** (optional `category` filter)

```bash
curl -s "$API/geofences?category=delivery_zone"
```

**Register a vehicle**

```bash
curl -s -X POST $API/vehicles -H 'Content-Type: application/json' -d '{
  "vehicle_number": "KA-01-AB-1234",
  "driver_name": "John Doe",
  "vehicle_type": "truck",
  "phone": "+1234567890"
}'
# {"id":"veh_1","vehicle_number":"KA-01-AB-1234","status":"active","time_ns":"..."}
```

**List vehicles**

```bash
curl -s $API/vehicles
```

**Configure an alert rule** (`vehicle_id` is optional; without it the rule covers all vehicles)

```bash
curl -s -X POST $API/alerts/configure -H 'Content-Type: application/json' -d '{
  "geofence_id": "geo_1",
  "vehicle_id": "veh_1",
  "event_type": "both"
}'
```

**List alert rules** (optional `geofence_id`, `vehicle_id` filters)

```bash
curl -s "$API/alerts?geofence_id=geo_1"
```

**Update a vehicle's location** (first outside, then inside: the second call is an entry)

```bash
curl -s -X POST $API/vehicles/location -H 'Content-Type: application/json' -d '{
  "vehicle_id": "veh_1", "latitude": 12.80, "longitude": 77.40, "timestamp": "2026-11-12T10:00:00Z"
}'
curl -s -X POST $API/vehicles/location -H 'Content-Type: application/json' -d '{
  "vehicle_id": "veh_1", "latitude": 12.95, "longitude": 77.55, "timestamp": "2026-11-12T10:01:00Z"
}'
# {"vehicle_id":"veh_1","location_updated":true,
#  "current_geofences":[{"geofence_id":"geo_1","geofence_name":"Downtown Delivery Zone","status":"inside"}],
#  "time_ns":"..."}
```

**Current location of a vehicle**

```bash
curl -s $API/vehicles/location/veh_1
```

**Violation history** (optional `vehicle_id`, `geofence_id`, `start_date`, `end_date`, `limit` up to 500, `offset`)

```bash
curl -s "$API/violations/history?vehicle_id=veh_1&limit=10"
```

**Watch live alerts** (needs Node.js 22+; run it, then send location updates from another terminal)

```bash
node -e 'const ws = new WebSocket("ws://localhost:8080/ws/alerts"); ws.onmessage = (e) => console.log(e.data)'
```

**Invalid input returns 400 with a message**

```bash
curl -s -X POST $API/geofences -H 'Content-Type: application/json' -d '{
  "name": "Open polygon", "category": "toll_zone",
  "coordinates": [[0,0],[0,1],[1,1],[1,0]]
}'
# {"error":"first and last coordinates must be identical (closed polygon)","time_ns":"..."}
```

Status codes used: `200` OK, `201` created, `400` invalid input, `404` unknown id or endpoint,
`409` duplicate (vehicle number or alert rule), `500` server error.

## 5. Frontend usage guide

The screen has three parts: a panel with four tabs on the left, the map in the middle, and the live alert feed on the right.

1. **Geofences tab.** Enter a name and category, press **Draw on map**, and click the map once for each corner (at least 3). Press **Create geofence**. The polygon is closed automatically. Coordinates can also be typed under *Or type coordinates*. The list below the form can be filtered by category; **Show** zooms the map to a geofence.
2. **Vehicles tab.** Register a vehicle. Then choose it under *Update location*, press **Move by clicking the map**, and click anywhere on the map: each click sends one location update. A position and time can also be typed by hand. The list shows each vehicle's last position and the geofences it is inside.
3. **Alert rules tab.** Choose a geofence, a vehicle (or *All vehicles*) and the event (entry, exit or both). Rules can be removed from the list.
4. **History tab.** All entry and exit events, with filters for vehicle, geofence and date range, 10 per page.
5. **Live alerts.** When a vehicle crosses a geofence that has a matching rule, a notice pops up and the alert is added to the feed, in every open browser tab. The badge at the top right shows whether the live connection is up; it reconnects by itself.

To see an alert quickly: create a geofence with the category *Restricted zone*, register a vehicle, and click inside the polygon while moving the vehicle.

## 6. Architecture overview

```
Browser (React + Leaflet)
   |  HTTP (JSON)                      ^  WebSocket /ws/alerts
   v                                   |
Go backend
   internal/api    HTTP handlers: validate input, write JSON with time_ns
   internal/store  all SQL (pgx connection pool)
   internal/ws     hub: tracks connected clients and broadcasts alerts
   |
   v
PostgreSQL + PostGIS (polygons stored as geometry, GiST spatial index)
```

What happens on `POST /vehicles/location`:

1. The handler validates the input.
2. The store opens a transaction and locks the vehicle's row.
3. The location is saved to the history table.
4. PostGIS finds the geofences that cover the point (`ST_Covers`, using the spatial index).
5. That set is compared with the set the vehicle was in before. New ones are entries, missing ones are exits.
6. Each event is saved. It is marked for alerting when a rule matches, or when a vehicle enters a restricted zone.
7. The transaction commits. The handler puts the alerts on the hub's queue and answers at once.
8. The hub's goroutine sends each alert to every connected browser.

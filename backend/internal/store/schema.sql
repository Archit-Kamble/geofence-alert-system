-- Database schema. Runs at every startup; every statement is safe to repeat.

CREATE EXTENSION IF NOT EXISTS postgis;

-- A geofence is a polygon on the map.
-- "coordinates" keeps the original [latitude, longitude] pairs exactly as the
-- client sent them. "boundary" is the same polygon as a PostGIS geometry
-- (longitude first), which is what the spatial queries use.
CREATE TABLE IF NOT EXISTS geofences (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    category    TEXT        NOT NULL CHECK (category IN
                    ('delivery_zone', 'restricted_zone', 'toll_zone', 'customer_area')),
    coordinates JSONB       NOT NULL,
    boundary    geometry(Polygon, 4326) NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Spatial index: lets "which polygons cover this point?" skip most rows.
CREATE INDEX IF NOT EXISTS geofences_boundary_idx ON geofences USING GIST (boundary);
CREATE INDEX IF NOT EXISTS geofences_category_idx ON geofences (category);

CREATE TABLE IF NOT EXISTS vehicles (
    id             BIGSERIAL PRIMARY KEY,
    vehicle_number TEXT        NOT NULL UNIQUE,
    driver_name    TEXT        NOT NULL,
    vehicle_type   TEXT        NOT NULL,
    phone          TEXT        NOT NULL,
    status         TEXT        NOT NULL DEFAULT 'active',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Every location update is kept (movement history).
CREATE TABLE IF NOT EXISTS vehicle_locations (
    id          BIGSERIAL PRIMARY KEY,
    vehicle_id  BIGINT           NOT NULL REFERENCES vehicles (id),
    latitude    DOUBLE PRECISION NOT NULL,
    longitude   DOUBLE PRECISION NOT NULL,
    recorded_at TIMESTAMPTZ      NOT NULL,
    created_at  TIMESTAMPTZ      NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS vehicle_locations_latest_idx
    ON vehicle_locations (vehicle_id, id DESC);

-- The geofences each vehicle is inside right now.
-- Entry and exit are found by comparing this set with the new position.
CREATE TABLE IF NOT EXISTS vehicle_geofence_state (
    vehicle_id  BIGINT      NOT NULL REFERENCES vehicles (id),
    geofence_id BIGINT      NOT NULL REFERENCES geofences (id),
    entered_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (vehicle_id, geofence_id)
);

-- Alert rules: "tell me when this vehicle (or any vehicle) enters/exits this geofence".
CREATE TABLE IF NOT EXISTS alert_rules (
    id          BIGSERIAL PRIMARY KEY,
    geofence_id BIGINT      NOT NULL REFERENCES geofences (id),
    vehicle_id  BIGINT      REFERENCES vehicles (id), -- NULL means all vehicles
    event_type  TEXT        NOT NULL CHECK (event_type IN ('entry', 'exit', 'both')),
    status      TEXT        NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The same rule cannot be saved twice. COALESCE turns "all vehicles" (NULL) into 0
-- so that two NULL rules also count as duplicates.
CREATE UNIQUE INDEX IF NOT EXISTS alert_rules_unique_idx
    ON alert_rules (geofence_id, COALESCE(vehicle_id, 0), event_type);

-- Every entry and exit, whether or not an alert was sent for it.
-- "alerted" is true when the event matched a rule and was pushed to clients.
CREATE TABLE IF NOT EXISTS geofence_events (
    id          BIGSERIAL PRIMARY KEY,
    vehicle_id  BIGINT           NOT NULL REFERENCES vehicles (id),
    geofence_id BIGINT           NOT NULL REFERENCES geofences (id),
    event_type  TEXT             NOT NULL CHECK (event_type IN ('entry', 'exit')),
    latitude    DOUBLE PRECISION NOT NULL,
    longitude   DOUBLE PRECISION NOT NULL,
    occurred_at TIMESTAMPTZ      NOT NULL,
    alerted     BOOLEAN          NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ      NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS geofence_events_time_idx     ON geofence_events (occurred_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS geofence_events_vehicle_idx  ON geofence_events (vehicle_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS geofence_events_geofence_idx ON geofence_events (geofence_id, occurred_at DESC);

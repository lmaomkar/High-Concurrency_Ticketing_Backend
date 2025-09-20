-- +migrate Up
-- init.sql - Evently schema with enhancements and partitioning (sharding via Postgres partitions)

-- Extensions
CREATE EXTENSION IF NOT EXISTS pgcrypto;   -- gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pg_trgm;

--------------------------------------------------------------------------------
-- Helper: function to update updated_at timestamp automatically
--------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION set_updated_at_column()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$;

--------------------------------------------------------------------------------
-- USERS
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL DEFAULT '',
    email TEXT UNIQUE NOT NULL,
    phone TEXT NOT NULL DEFAULT '',
    -- Authentication
    password_hash TEXT NOT NULL DEFAULT '',                    -- for local auth (bcrypt)
    oauth_provider TEXT NOT NULL DEFAULT '',                   -- e.g. 'google'
    oauth_sub TEXT NOT NULL DEFAULT '',                        -- provider-specific user id
    role TEXT CHECK (role IN ('user','admin')) DEFAULT 'user',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_admin_users ON users(id) WHERE role = 'admin';

-- trigger to update updated_at
CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION set_updated_at_column();

--------------------------------------------------------------------------------
-- EVENTS
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    venue TEXT NOT NULL DEFAULT '',
    start_time TIMESTAMPTZ,
    end_time TIMESTAMPTZ,
    category TEXT,                           -- e.g., concert, conference
    capacity INT NOT NULL,
    reserved INT NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    status TEXT CHECK (status IN ('upcoming','ongoing','cancelled','expired')) DEFAULT 'upcoming' NOT NULL,
    seats JSONB NOT NULL DEFAULT '[]',
    ticket_price NUMERIC(12,2) DEFAULT 0,    -- base charge per seat
    cancellation_fee NUMERIC(12,2) DEFAULT 0,-- absolute fee (app logic can treat as percent)
    likes INT NOT NULL DEFAULT 0,            -- denormalized count (also track details in event_likes)
    maximum_tickets_per_booking INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_events_start_time ON events(start_time);
CREATE INDEX IF NOT EXISTS idx_events_name_trgm ON events USING gin (name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_events_likes ON events(likes);

-- trigger to update updated_at
CREATE TRIGGER events_set_updated_at BEFORE UPDATE ON events
FOR EACH ROW EXECUTE FUNCTION set_updated_at_column();

--------------------------------------------------------------------------------
-- EVENT_LIKES (track who liked which event)
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS event_likes (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    event_id UUID REFERENCES events(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (user_id, event_id)
);
CREATE INDEX IF NOT EXISTS idx_event_likes_event ON event_likes(event_id);

--------------------------------------------------------------------------------
-- EVENT_CAPACITY - partitioned by event_id (hash) so capacity/reserved_count are sharded
--------------------------------------------------------------------------------
-- Create partitioned parent
CREATE TABLE IF NOT EXISTS event_capacity (
    event_id UUID NOT NULL,
    capacity INT NOT NULL,
    reserved_count INT NOT NULL DEFAULT 0,
    held_count INT NOT NULL DEFAULT 0,    -- pending holds waiting for finalization
    updated_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (event_id)
) PARTITION BY HASH (event_id);

-- Create N partitions (example: 4). Increase as required.
CREATE TABLE IF NOT EXISTS event_capacity_p0 PARTITION OF event_capacity FOR VALUES WITH (modulus 4, remainder 0);
CREATE TABLE IF NOT EXISTS event_capacity_p1 PARTITION OF event_capacity FOR VALUES WITH (modulus 4, remainder 1);
CREATE TABLE IF NOT EXISTS event_capacity_p2 PARTITION OF event_capacity FOR VALUES WITH (modulus 4, remainder 2);
CREATE TABLE IF NOT EXISTS event_capacity_p3 PARTITION OF event_capacity FOR VALUES WITH (modulus 4, remainder 3);

-- trigger to update updated_at on parent partitions
CREATE TRIGGER event_capacity_set_updated_at
BEFORE UPDATE ON event_capacity
FOR EACH ROW EXECUTE FUNCTION set_updated_at_column();

--------------------------------------------------------------------------------
-- BOOKINGS - partitioned/sharded by event_id (hash)
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS bookings (
    id UUID DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    event_id UUID REFERENCES events(id) ON DELETE SET NULL,
    status TEXT CHECK (status IN ('pending','booked','cancelled','waitlisted','expired')) NOT NULL,
    seats JSONB NULL,
    idempotency_key TEXT,
    amount_paid NUMERIC(12,2) DEFAULT 0,
    payment_status TEXT CHECK (payment_status IN ('pending','paid','failed','refunded')) DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    version INT DEFAULT 1,
    CONSTRAINT unique_event_idempotency UNIQUE (event_id, idempotency_key),
    PRIMARY KEY(event_id, id)
) PARTITION BY HASH (event_id);

-- create 4 partitions (example)
CREATE TABLE IF NOT EXISTS bookings_p0 PARTITION OF bookings FOR VALUES WITH (modulus 4, remainder 0);
CREATE TABLE IF NOT EXISTS bookings_p1 PARTITION OF bookings FOR VALUES WITH (modulus 4, remainder 1);
CREATE TABLE IF NOT EXISTS bookings_p2 PARTITION OF bookings FOR VALUES WITH (modulus 4, remainder 2);
CREATE TABLE IF NOT EXISTS bookings_p3 PARTITION OF bookings FOR VALUES WITH (modulus 4, remainder 3);

-- helpful indexes (created on parent are propagated to partitions in PG14+, but adding explicitly for safety)
CREATE INDEX IF NOT EXISTS idx_bookings_event_status ON bookings (event_id, status);
CREATE INDEX IF NOT EXISTS idx_bookings_user ON bookings (user_id);
-- trigger to update updated_at on bookings
CREATE TRIGGER bookings_set_updated_at BEFORE UPDATE ON bookings
FOR EACH ROW EXECUTE FUNCTION set_updated_at_column();

--------------------------------------------------------------------------------
-- WAITLIST - partitioned by event_id (hash)
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS waitlist (
    id UUID DEFAULT gen_random_uuid(),
    event_id UUID REFERENCES events(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    position INT NOT NULL,
    opted_out BOOLEAN DEFAULT FALSE,
    notified_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY(event_id, id)
) PARTITION BY HASH (event_id);

CREATE TABLE IF NOT EXISTS waitlist_p0 PARTITION OF waitlist FOR VALUES WITH (modulus 4, remainder 0);
CREATE TABLE IF NOT EXISTS waitlist_p1 PARTITION OF waitlist FOR VALUES WITH (modulus 4, remainder 1);
CREATE TABLE IF NOT EXISTS waitlist_p2 PARTITION OF waitlist FOR VALUES WITH (modulus 4, remainder 2);
CREATE TABLE IF NOT EXISTS waitlist_p3 PARTITION OF waitlist FOR VALUES WITH (modulus 4, remainder 3);

CREATE INDEX IF NOT EXISTS idx_waitlist_event_position ON waitlist (event_id, position);

--------------------------------------------------------------------------------
-- SEATS - partitioned by event_id for seat-level booking scalability
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS seats (
    id UUID DEFAULT gen_random_uuid(),
    event_id UUID REFERENCES events(id) ON DELETE CASCADE,
    seat_label TEXT,
    status TEXT CHECK (status IN ('available','held','booked')) DEFAULT 'available',
    held_until TIMESTAMPTZ NULL,
    held_by_booking UUID NULL,             -- references bookings.id across partitions (no FK enforced across partitions here)
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY(event_id, id)
) PARTITION BY HASH (event_id);

CREATE TABLE IF NOT EXISTS seats_p0 PARTITION OF seats FOR VALUES WITH (modulus 4, remainder 0);
CREATE TABLE IF NOT EXISTS seats_p1 PARTITION OF seats FOR VALUES WITH (modulus 4, remainder 1);
CREATE TABLE IF NOT EXISTS seats_p2 PARTITION OF seats FOR VALUES WITH (modulus 4, remainder 2);
CREATE TABLE IF NOT EXISTS seats_p3 PARTITION OF seats FOR VALUES WITH (modulus 4, remainder 3);

CREATE INDEX IF NOT EXISTS idx_seats_event_label ON seats (event_id, seat_label);

-- trigger to update updated_at on seats
CREATE TRIGGER seats_set_updated_at BEFORE UPDATE ON seats
FOR EACH ROW EXECUTE FUNCTION set_updated_at_column();

--------------------------------------------------------------------------------
-- BOOKING_AUDIT - immutable audit log for analytics
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS booking_audit (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_id UUID,
    event_id UUID,
    user_id UUID,
    action TEXT CHECK (action IN ('created','cancelled','waitlisted','expired','finalized','promoted')) NOT NULL,
    payload JSONB NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_booking_audit_event ON booking_audit (event_id);
CREATE INDEX IF NOT EXISTS idx_booking_audit_booking ON booking_audit (booking_id);

--------------------------------------------------------------------------------
-- ANALYTICS AGGREGATES (time-bucketed)
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS analytics_aggregates (
    event_id UUID,
    day DATE,
    total_bookings INT DEFAULT 0,
    cancellations INT DEFAULT 0,
    capacity_utilization NUMERIC(5,2) DEFAULT 0, -- percent
    PRIMARY KEY (event_id, day)
);

--------------------------------------------------------------------------------
-- OTHER HELPERS / ADMIN TABLES
--------------------------------------------------------------------------------
-- simple admin user table is covered under users.role = 'admin'. If you prefer separate table:
-- CREATE TABLE IF NOT EXISTS admins (...) 

-- generic key-value for feature flags or metadata
CREATE TABLE IF NOT EXISTS kv_store (
    key TEXT PRIMARY KEY,
    value JSONB,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);
CREATE TRIGGER kv_store_set_updated_at BEFORE UPDATE ON kv_store
FOR EACH ROW EXECUTE FUNCTION set_updated_at_column();

--------------------------------------------------------------------------------
-- Views and materialized views (optional)
--------------------------------------------------------------------------------
-- Example: top events by bookings (materialized refreshable)
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_top_events AS
SELECT e.id, e.name, COUNT(b.*) as bookings
FROM events e
LEFT JOIN bookings b ON e.id = b.event_id AND b.status = 'booked'
GROUP BY e.id, e.name
ORDER BY bookings DESC
WITH NO DATA;

--------------------------------------------------------------------------------
-- CONSTRAINTS & NOTES
--------------------------------------------------------------------------------
-- Note: because bookings/seats/waitlist are partitioned by event_id using HASH,
-- lookups by event_id will be routed to the right partition automatically.
-- If you plan to scale further across multiple physical DB servers (multi-DB sharding),
-- you would implement a mapping layer in the application and point event_id -> DB instance.

--------------------------------------------------------------------------------
-- Example helper functions (reconciliation)
--------------------------------------------------------------------------------
-- A simple function to get remaining tokens computed from event_capacity or fallback to events table.
CREATE OR REPLACE FUNCTION event_remaining_capacity(eid UUID) RETURNS INT LANGUAGE sql AS $$
SELECT capacity - reserved_count FROM event_capacity WHERE event_id = eid
UNION ALL
SELECT (capacity - reserved) FROM events WHERE id = eid
LIMIT 1;
$$;


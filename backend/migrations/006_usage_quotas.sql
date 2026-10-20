-- Migration 006: Persistent daily usage quotas for paid AI features.
-- Counters include in-flight reservations so restarts and replicas cannot
-- reset or multiply an allowance. Days are UTC calendar dates.

CREATE TABLE IF NOT EXISTS usage_counters (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    feature TEXT NOT NULL,
    usage_day DATE NOT NULL,
    used_units BIGINT NOT NULL DEFAULT 0 CHECK (used_units >= 0),
    PRIMARY KEY (user_id, feature, usage_day)
);

-- Global beta budget per feature. Not owned by a user, so account deletion
-- never returns cost that may already have been incurred.
CREATE TABLE IF NOT EXISTS usage_global_counters (
    feature TEXT NOT NULL,
    usage_day DATE NOT NULL,
    used_units BIGINT NOT NULL DEFAULT 0 CHECK (used_units >= 0),
    PRIMARY KEY (feature, usage_day)
);

-- One row per admitted operation: its charge, settlement and provider usage.
CREATE TABLE IF NOT EXISTS usage_reservations (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    feature TEXT NOT NULL,
    usage_day DATE NOT NULL,
    units BIGINT NOT NULL CHECK (units > 0),
    status TEXT NOT NULL DEFAULT 'reserved'
        CHECK (status IN ('reserved', 'settled', 'failed', 'released')),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    settled_at TIMESTAMP WITH TIME ZONE,
    gateway TEXT,
    region TEXT,
    model TEXT,
    attempts INTEGER,
    input_tokens BIGINT,
    output_tokens BIGINT,
    total_tokens BIGINT,
    characters BIGINT,
    provider_cost NUMERIC(18, 8),
    currency TEXT,
    cost_status TEXT CHECK (cost_status IN ('reported', 'missing', 'none')),
    latency_ms BIGINT
);

CREATE INDEX IF NOT EXISTS idx_usage_reservations_status_created
    ON usage_reservations(status, created_at);

CREATE INDEX IF NOT EXISTS idx_usage_reservations_user_id
    ON usage_reservations(user_id);

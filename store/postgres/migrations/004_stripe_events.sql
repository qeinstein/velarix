CREATE TABLE IF NOT EXISTS stripe_processed_events (
    event_id    TEXT    PRIMARY KEY,
    processed_at BIGINT NOT NULL
);

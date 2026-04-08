CREATE TABLE IF NOT EXISTS semantic_fact_embeddings (
    org_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    fact_id TEXT NOT NULL,
    updated_at BIGINT NOT NULL,
    resolved_status DOUBLE PRECISION NOT NULL DEFAULT 0,
    embedding_json JSONB NOT NULL,
    doc JSONB NOT NULL,
    PRIMARY KEY (org_id, session_id, fact_id)
);

CREATE INDEX IF NOT EXISTS idx_semantic_fact_embeddings_lookup
    ON semantic_fact_embeddings (org_id, session_id, updated_at DESC, fact_id DESC);

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = 'vector') THEN
        CREATE EXTENSION IF NOT EXISTS vector;
        IF NOT EXISTS (
            SELECT 1
            FROM information_schema.columns
            WHERE table_name = 'semantic_fact_embeddings' AND column_name = 'embedding_vector'
        ) THEN
            ALTER TABLE semantic_fact_embeddings
                ADD COLUMN embedding_vector vector(128);
        END IF;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'semantic_fact_embeddings' AND column_name = 'embedding_vector'
    ) THEN
        EXECUTE 'CREATE INDEX IF NOT EXISTS idx_semantic_fact_embeddings_ivfflat
                 ON semantic_fact_embeddings
                 USING ivfflat (embedding_vector vector_cosine_ops)
                 WITH (lists = 100)';
    END IF;
END $$;

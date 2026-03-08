-- Edge case test fixture

-- Table with reserved Go keywords as names
CREATE TABLE "select" (
    "type" SERIAL PRIMARY KEY,
    "map" TEXT NOT NULL,
    "range" INTEGER,
    "func" BOOLEAN NOT NULL DEFAULT false
);

-- Table with no columns besides PK (minimal)
CREATE TABLE empty_ish (
    id SERIAL PRIMARY KEY
);

-- Table with all nullable columns (except PK)
CREATE TABLE all_nullable (
    id SERIAL PRIMARY KEY,
    a TEXT,
    b INTEGER,
    c BOOLEAN,
    d TIMESTAMPTZ,
    e JSONB
);

-- Self-referencing with unique constraint (HasOne self)
CREATE TABLE tree_nodes (
    id SERIAL PRIMARY KEY,
    parent_id INTEGER UNIQUE REFERENCES tree_nodes(id),
    label TEXT NOT NULL
);

-- Table with composite primary key (non-join table)
CREATE TABLE versioned_docs (
    doc_id UUID NOT NULL,
    version INTEGER NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (doc_id, version)
);

-- Table with multiple FKs to the same table
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id UUID NOT NULL,
    recipient_id UUID NOT NULL
);

-- Various PG types
CREATE TABLE type_zoo (
    id SERIAL PRIMARY KEY,
    a_smallint SMALLINT NOT NULL,
    a_bigint BIGINT NOT NULL,
    a_real REAL NOT NULL,
    a_double DOUBLE PRECISION NOT NULL,
    a_numeric NUMERIC(10,2) NOT NULL,
    a_bool BOOLEAN NOT NULL,
    a_text TEXT NOT NULL,
    a_varchar VARCHAR(255) NOT NULL,
    a_char CHAR(1) NOT NULL,
    a_bytea BYTEA NOT NULL,
    a_timestamp TIMESTAMP NOT NULL,
    a_timestamptz TIMESTAMPTZ NOT NULL,
    a_date DATE NOT NULL,
    a_time TIME NOT NULL,
    a_uuid UUID NOT NULL,
    a_json JSON NOT NULL,
    a_jsonb JSONB NOT NULL,
    a_inet INET NOT NULL,
    a_text_array TEXT[] NOT NULL,
    a_int_array INTEGER[] NOT NULL
);

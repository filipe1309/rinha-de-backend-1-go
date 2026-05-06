CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE IF NOT EXISTS pessoas (
    id           UUID PRIMARY KEY,
    apelido      VARCHAR(32) UNIQUE NOT NULL,
    nome         VARCHAR(100) NOT NULL,
    nascimento   DATE NOT NULL,
    stack        TEXT,
    search_field TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_pessoas_search ON pessoas USING GIN (search_field gin_trgm_ops);

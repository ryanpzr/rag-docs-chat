CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE documents (
    id          BIGSERIAL PRIMARY KEY,
    filename    TEXT        NOT NULL,
    content     TEXT        NOT NULL,
    size_bytes  INTEGER     NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The vector dimension must match the embeddings model
-- (nomic-embed-text = 768). Switching to a model with another
-- dimension requires a new migration.
CREATE TABLE chunks (
    id           BIGSERIAL PRIMARY KEY,
    document_id  BIGINT      NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    chunk_index  INTEGER     NOT NULL,
    content      TEXT        NOT NULL,
    embedding    vector(768) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (document_id, chunk_index)
);

-- Approximate nearest-neighbour index for cosine similarity search
-- (queries ordered by `embedding <=> $1`).
CREATE INDEX chunks_embedding_hnsw_idx
    ON chunks USING hnsw (embedding vector_cosine_ops);

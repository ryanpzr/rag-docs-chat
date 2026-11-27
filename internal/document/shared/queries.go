package shared

const insertDocument = `
INSERT INTO documents (filename, content, size_bytes)
VALUES ($1, $2, $3)
RETURNING id, created_at`

const listSummaries = `
SELECT d.id, d.filename, d.size_bytes, COUNT(c.id)::int AS chunk_count, d.created_at
FROM documents d
LEFT JOIN chunks c ON c.document_id = d.id
GROUP BY d.id
ORDER BY d.created_at DESC, d.id DESC`

// For a vector column, atttypmod holds the declared dimension.
const embeddingDimensions = `
SELECT atttypmod
FROM pg_attribute
WHERE attrelid = 'chunks'::regclass AND attname = 'embedding'`

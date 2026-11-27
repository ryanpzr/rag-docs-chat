# rag-docs-chat

RAG: ask questions about your documents (Go + pgvector + Claude).

A learning project that builds Retrieval-Augmented Generation step by step:

1. **Ingestion** (done): upload a document, split it into chunks, embed each chunk and store it in Postgres + pgvector.
2. Retrieval: top-K similarity search endpoint.
3. Generation: answer questions with Claude using the retrieved chunks, with citations.
4. Frontend: chat UI plus a panel showing the retrieved chunks and their scores.
5. Extras: hybrid search, reranking, streaming.

## Stack

| Piece        | Choice                                                         |
| ------------ | -------------------------------------------------------------- |
| API          | Go, standard library `net/http`                                |
| Database     | Postgres 17 + [pgvector](https://github.com/pgvector/pgvector) |
| Migrations   | [golang-migrate](https://github.com/golang-migrate/migrate)    |
| Embeddings   | [Ollama](https://ollama.com) running `nomic-embed-text` (768 dimensions), locally and free |

The embeddings provider sits behind the `embedder.Embedder` interface
([pkg/embedder](pkg/embedder/embedder.go)), so Voyage or OpenAI can be added
later as another implementation selected by `EMBEDDINGS_PROVIDER`.

## Project layout

```
cmd/api/                     entry point
internal/app/                wiring and routes
internal/config/             env var configuration
internal/document/
  domain/                    Document, Chunk
  shared/                    repository (SQL only, no business rules)
  features/
    ingest_document/         POST /documents: handler -> usecase
    list_documents/          GET /documents:  handler -> usecase
pkg/chunker/                 text splitting with overlap
pkg/embedder/                Embedder interface + ollama implementation
pkg/apperrors/               errors with HTTP status, JSON error responses
db/migrations/               SQL migrations
```

Handlers parse HTTP, usecases validate input and decide which errors to
return, and repositories only talk to the database.

## How ingestion works

1. `POST /documents` receives a `.txt` or `.md` file (multipart field `file`).
2. The usecase checks the extension, size, UTF-8 encoding and that the file is not empty.
3. The chunker splits the text into chunks of `CHUNK_SIZE` characters, where
   consecutive chunks share about `CHUNK_OVERLAP` characters. It prefers to cut
   at paragraph, line, sentence or word boundaries.
4. Every chunk is embedded by Ollama (in batches of `EMBEDDINGS_BATCH_SIZE`).
5. The document and its chunks are stored in one transaction. `chunks.embedding`
   is a `vector(768)` column with an HNSW index for cosine similarity.

## Running locally

Requirements: Docker (with Compose) and Go 1.25+.

```bash
cp .env.example .env
docker compose up -d
```

This starts:

- `db`: Postgres + pgvector on `localhost:5433`
- `migrate`: applies `db/migrations` and exits
- `ollama`: embeddings server on `localhost:11434`
- `ollama-pull`: downloads `nomic-embed-text` (~270 MB, first run only) and exits

Check that the one-off jobs finished:

```bash
docker compose ps -a          # migrate and ollama-pull should show "Exited (0)"
docker compose logs migrate   # "1/u create_documents_and_chunks"
```

Then start the API:

```bash
go run ./cmd/api
```

The API listens on `:8080`. If that port is taken, set another one in `.env`
(for example `HTTP_ADDR=:8081`) and adjust the curl examples.

### Migrations

Migrations run automatically with `docker compose up`. To run them by hand:

```bash
docker compose run --rm migrate                 # apply all
docker compose run --rm migrate \
  -path=/migrations \
  -database="postgres://rag:rag@db:5432/rag?sslmode=disable" \
  down 1                                        # roll back the last one
```

The embedding dimension is fixed in the migration (`vector(768)`). If you
switch to a model with another dimension, add a migration and set
`EMBEDDINGS_DIMENSIONS`. The API refuses to start when they do not match.

## API

### Upload a document

```bash
curl -F "file=@notes.md" http://localhost:8080/documents
```

```json
{
  "id": 1,
  "filename": "notes.md",
  "size_bytes": 5120,
  "chunk_count": 7,
  "created_at": "2026-11-26T22:45:00Z"
}
```

Errors are JSON `{"error": "..."}` with status `400` (empty, not UTF-8,
missing file), `413` (larger than `MAX_UPLOAD_BYTES`), `415` (not `.txt` or
`.md`) or `502` (embeddings provider unavailable).

### List documents

```bash
curl http://localhost:8080/documents
```

```json
{
  "documents": [
    {
      "id": 1,
      "filename": "notes.md",
      "size_bytes": 5120,
      "chunk_count": 7,
      "created_at": "2026-11-26T22:45:00Z"
    }
  ]
}
```

### Inspect the stored chunks

```bash
docker compose exec db psql -U rag -d rag -c \
  "SELECT document_id, chunk_index, left(content, 60) AS content, vector_dims(embedding) FROM chunks ORDER BY 1, 2;"
```

## Configuration

All settings come from environment variables (a `.env` file is loaded if
present). See [.env.example](.env.example).

| Variable                | Default                                                  | Description                         |
| ----------------------- | -------------------------------------------------------- | ----------------------------------- |
| `HTTP_ADDR`             | `:8080`                                                  | API listen address                  |
| `DATABASE_URL`          | `postgres://rag:rag@localhost:5433/rag?sslmode=disable`  | Postgres connection string          |
| `MAX_UPLOAD_BYTES`      | `5242880`                                                | Largest accepted file (5 MB)        |
| `CHUNK_SIZE`            | `1000`                                                   | Max characters per chunk            |
| `CHUNK_OVERLAP`         | `200`                                                    | Characters shared by adjacent chunks |
| `EMBEDDINGS_PROVIDER`   | `ollama`                                                 | Embeddings implementation           |
| `EMBEDDINGS_MODEL`      | `nomic-embed-text`                                       | Model name                          |
| `EMBEDDINGS_DIMENSIONS` | `768`                                                    | Must match `chunks.embedding`       |
| `EMBEDDINGS_BATCH_SIZE` | `32`                                                     | Chunks per embeddings request       |
| `OLLAMA_URL`            | `http://localhost:11434`                                 | Ollama server                       |

## Tests

```bash
go test ./...
```

Unit tests cover the chunker, the ingestion usecase (with fake embedder and
repository) and the Ollama client (with a fake HTTP server).

## License

MIT

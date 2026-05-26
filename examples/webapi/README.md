# Gocene `webapi` example

A small, self-contained HTTP service that demonstrates how to drive the
Gocene module end-to-end: open an on-disk index, index documents through
`IndexWriter`, run paginated queries through the classic query parser, and
seed the index from a golden corpus shipped with the binary.

The domain modelled by the demo is a list of `Book` records.

## Running

From the repository root:

```bash
go run ./examples/webapi/cmd/webapi
```

Flags:

| Flag    | Default                                  | Description                          |
|---------|------------------------------------------|--------------------------------------|
| `-addr` | `:8080`                                  | HTTP listen address.                 |
| `-data` | `<os.TempDir>/gocene-webapi-index`       | Directory holding the Gocene index.  |

On the first boot (when the index is empty) the embedded golden corpus
under `books.json` is loaded automatically — 12 deterministic books across
varied authors, years and tags.

## Endpoints

| Method   | Path             | Description                                        |
|----------|------------------|----------------------------------------------------|
| `GET`    | `/healthz`       | Liveness probe (`{"status":"ok"}`).                |
| `POST`   | `/books`         | Create a book. Returns 201 with the assigned `id`. |
| `GET`    | `/books/{id}`    | Read a single book by id. Returns 404 if missing.  |
| `PUT`    | `/books/{id}`    | Replace the book with the given id.                |
| `DELETE` | `/books/{id}`    | Remove a book. Returns 204.                        |
| `GET`    | `/books`         | Paginated search (see below).                      |

### Search parameters

| Parameter | Default                                | Description                                                                |
|-----------|----------------------------------------|----------------------------------------------------------------------------|
| `q`       | — (matches everything when omitted)    | Lucene-style query string.                                                 |
| `field`   | empty (multi-field over `SearchableFields`) | Restrict the search to a single field. Valid values: `id`, `title`, `author`, `year`, `tags`, `summary`. |
| `page`    | `1`                                    | 1-based page index.                                                        |
| `size`    | `10` (max `100`)                       | Page size.                                                                 |

The response shape is

```json
{
  "total": 5,
  "page":  1,
  "size":  10,
  "items": [ { "id": "...", "title": "...", ... } ]
}
```

### `curl` examples

```bash
# List the first 5 seeded books.
curl -s 'http://localhost:8080/books?size=5' | jq

# Full-text search across title/author/tags/summary.
curl -s 'http://localhost:8080/books?q=lucene' | jq

# Restrict to one field.
curl -s 'http://localhost:8080/books?q=Martin&field=author' | jq

# Paginate.
curl -s 'http://localhost:8080/books?q=programming&field=title&page=2&size=3' | jq

# Filter by exact year.
curl -s 'http://localhost:8080/books?q=1999&field=year' | jq

# Create.
curl -s -X POST 'http://localhost:8080/books' \
  -H 'Content-Type: application/json' \
  -d '{"title":"Gocene in Action","author":"Flavio Oliveira","year":2026,
       "tags":["gocene","go","lucene"],
       "summary":"Hands-on guide to Gocene."}' | jq

# Read by id (substitute the id returned by the POST above).
curl -s 'http://localhost:8080/books/book-XXXX' | jq

# Update.
curl -s -X PUT 'http://localhost:8080/books/book-XXXX' \
  -H 'Content-Type: application/json' \
  -d '{"title":"Gocene in Action (Revised)","author":"Flavio Oliveira",
       "year":2026,"tags":["gocene","revised"],"summary":"Revised."}' | jq

# Delete.
curl -s -X DELETE 'http://localhost:8080/books/book-XXXX' -o /dev/null -w '%{http_code}\n'
```

## How it uses Gocene

| Gocene component                                | Role in the demo                                                   |
|-------------------------------------------------|--------------------------------------------------------------------|
| `store.MMapDirectory`                           | On-disk directory backing the index.                               |
| `analysis.StandardAnalyzer`                     | Tokenisation pipeline for text fields and the query parsers.       |
| `index.IndexWriter` + `IndexWriterConfig`       | Indexing pipeline (Add/Delete/Commit).                             |
| `document.NewTextField` / `NewStringField`      | Per-field indexing layout (text vs exact terms, stored).           |
| `index.OpenDirectoryReader`                     | Snapshot reader opened fresh on every search request.              |
| `search.NewIndexSearcher`                       | Search entry point against the snapshot.                           |
| `search.MatchAllDocsQuery`, `search.TermQuery`  | Default and exact-match queries.                                   |
| `queryparser.QueryParser`, `MultiFieldQueryParser` | Parsing of the user-supplied `q=` parameter.                    |

## Known limitations (and why the demo still works)

The webapi was originally meant to demonstrate the canonical Gocene
write/read path end-to-end. During the implementation of sprint 115 we
hit two pre-existing gaps in the Gocene core that are out of scope for
this demo and are tracked as a separate roadmap item:

1. `IndexWriter` does not yet persist documents to disk. The default
   `IndexWriterConfig` does not assign a codec, and `DocumentsWriter.SetCodec`
   is never called from production code. As a result `DocumentsWriter.flush`
   silently drops the in-RAM documents (see
   `index/documents_writer.go:230-234`); only `segments_N`, `.si` and
   `write.lock` ever appear on disk.
2. `OpenDirectoryReader` builds each `SegmentReader` without initialising
   `SegmentCoreReaders`. Consequently `IndexSearcher.Doc(docID)` fails
   with `segment reader not initialized` (this is documented in
   `index/read_only_index_test.go:64-72`).

Both gaps are tracked by **rmp task 4636** under the `gocene` roadmap.

To keep the demo fully functional today, `BookStore` (see `store.go`)
maintains an in-memory shadow of every book and remaps each Gocene-internal
doc id back to its domain id through a dedicated ordered slice. The Gocene
index still drives full-text search (the in-memory `FieldsProducer` works
while the writer is alive); the shadow drives round-trip retrieval and
the exact-match fields (`id`, `year`). Once the upstream gaps are closed
this shadow layer can be removed and `BookStore.Get` / `BookStore.Search`
items can be hydrated directly from `IndexSearcher.Doc(docID)`.

The demo therefore behaves correctly from the outside (you can POST, GET,
PUT, DELETE and search through Gocene queries with paginated, deduplicated
results), with the caveat that retrieval is currently shadow-backed
rather than codec-backed.

## Testing

```bash
go test ./examples/webapi/... -count=1
```

The integration test under `webapi_test.go` brings up the HTTP server in
process and exercises the full CRUD lifecycle plus paginated search across
several fields. It is the contract the demo must keep green.

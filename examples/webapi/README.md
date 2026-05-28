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
| `index.OpenDirectoryReader`                     | Snapshot reader opened fresh on every read (Get, Search, IsEmpty). |
| `search.NewIndexSearcher` + `IndexSearcher.Doc` | Search entry point and stored-field retrieval that hydrates every `Book` straight from the index. |
| `search.MatchAllDocsQuery`, `search.TermQuery`  | Listing/default query and exact-match (`id`, `year`) queries.      |
| `queryparser.QueryParser`, `MultiFieldQueryParser` | Parsing of the user-supplied `q=` parameter.                    |
| `codecs.Lucene104Codec` (blank-imported)        | Production codec linked in so the writer persists segments to disk and the reader can hydrate stored fields. |

## Reads go through the live index

Every read in this demo is resolved against the Gocene index — there is no
in-memory shadow of `Book` data. `BookStore.Get` runs a `TermQuery` on the
`id` field and hydrates the result from its stored fields via
`IndexSearcher.Doc`; `BookStore.Search` does the same for every hit;
`BookStore.IsEmpty` reads `DirectoryReader.NumDocs`. The production
`Lucene104Codec` is blank-imported in `store.go` so the writer actually
persists segments and the reader can read them back.

### One remaining core gap: codec term-deletes

`Put` and `Delete` are expressed as an *index rebuild* rather than
`IndexWriter.UpdateDocument` / `DeleteDocuments`. The Gocene codec read path
does not yet apply buffered term-deletes to already-committed segments: a
`DeleteDocuments(term)` followed by `Commit` leaves the document visible to a
freshly opened reader, and an in-place `UpdateDocument` therefore duplicates
the document instead of replacing it (verified directly while implementing
rmp tasks 4665/4671 — a delete+commit left `NumDocs` unchanged and the term
still matched). Until that gap is closed, a mutation reads the current set of
live books back *from the index*, applies the change for the duration of the
call only, and re-creates the index from scratch in a fresh `CREATE` writer.
No `Book` is retained between calls, so the index remains the single source of
truth. This rebuild strategy is a property of the demo's `BookStore`, not of
the data model: the on-disk layout is the standard codec output and round-trips
through `IndexSearcher.Doc` unchanged.

## Testing

```bash
go test ./examples/webapi/... -count=1
```

The integration test under `webapi_test.go` brings up the HTTP server in
process and exercises the full CRUD lifecycle plus paginated search across
several fields. It is the contract the demo must keep green.

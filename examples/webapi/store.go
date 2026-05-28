// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package webapi

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	// Blank-import the production codec packages so their init() functions
	// register the default Lucene 10.4 codec (and the temporary stored-fields
	// format) with package index. Without these registrations
	// NewIndexWriterConfig leaves the codec nil and Commit silently drops the
	// in-RAM documents, so nothing would ever reach disk for the reader to
	// hydrate from. The whole point of this store is that every read goes
	// through the persisted index, which is only possible once the codec is
	// linked in.
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90/compressing"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queryparser"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ErrBookNotFound is returned by Get/Update/Delete when no document with the
// requested id is indexed.
var ErrBookNotFound = errors.New("book not found")

// ErrUnknownField is returned by Search when the field parameter is not a
// valid indexed field name.
var ErrUnknownField = errors.New("unknown search field")

// ErrBadQuery is returned by Search when the query string cannot be parsed.
var ErrBadQuery = errors.New("invalid query syntax")

// BookStore exposes a small CRUD + search surface tailored to the Book domain
// and backed exclusively by a Gocene index. The index is the single source of
// truth: every read (Get, Search, IsEmpty) is resolved against a freshly
// opened DirectoryReader and every Book is reconstructed from its stored
// fields via IndexSearcher.Doc — there is no in-memory shadow of Book data.
//
// Mutations (Put, Delete) are still expressed as an index rebuild rather than
// IndexWriter.UpdateDocument/DeleteDocuments. The Gocene codec read path does
// not yet apply buffered term-deletes to already-committed segments (a
// DeleteDocuments + Commit leaves the document visible to a freshly opened
// reader), so an in-place update would duplicate the document and a delete
// would be a no-op. To keep the demo correct, Put and Delete read the current
// set of live books back from the index, apply the change in memory for the
// duration of the call only, then re-create the index from scratch in a fresh
// writer. No Book is retained between calls.
type BookStore struct {
	dir      *store.MMapDirectory
	analyzer *analysis.StandardAnalyzer

	mu     sync.RWMutex
	writer *index.IndexWriter
}

// OpenBookStore opens (and if needed creates) an on-disk index at the given
// path. The caller owns the resulting BookStore and must call Close.
func OpenBookStore(path string) (*BookStore, error) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, fmt.Errorf("create index directory %q: %w", path, err)
	}

	dir, err := store.NewMMapDirectory(path)
	if err != nil {
		return nil, fmt.Errorf("open mmap directory %q: %w", path, err)
	}

	analyzer := analysis.NewStandardAnalyzer()
	cfg := index.NewIndexWriterConfig(analyzer)
	cfg.SetOpenMode(index.CREATE_OR_APPEND)
	writer, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		return nil, fmt.Errorf("open index writer at %q: %w", path, err)
	}

	return &BookStore{
		dir:      dir,
		analyzer: analyzer,
		writer:   writer,
	}, nil
}

// Close flushes any pending writes and releases the underlying resources.
func (s *BookStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var firstErr error
	if s.writer != nil {
		if err := s.writer.Commit(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("commit on close: %w", err)
		}
		if err := s.writer.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close writer: %w", err)
		}
		s.writer = nil
	}
	if s.dir != nil {
		if err := s.dir.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close directory: %w", err)
		}
		s.dir = nil
	}
	return firstErr
}

// IsEmpty reports whether the store currently holds zero books. The count is
// taken from the live reader (DirectoryReader.NumDocs), never from any
// in-memory state.
func (s *BookStore) IsEmpty() (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	reader, err := index.OpenDirectoryReader(s.dir)
	if err != nil {
		return false, fmt.Errorf("open reader: %w", err)
	}
	defer reader.Close()
	return reader.NumDocs() == 0, nil
}

// Put indexes a book. If book.ID is empty an id is generated and assigned in
// place. An existing id is replaced. The index is the source of truth: the
// current set of live books is read back from the index, the supplied book is
// upserted into that set, and the index is rebuilt from the result.
func (s *BookStore) Put(book *Book) error {
	if book.ID == "" {
		id, err := generateID()
		if err != nil {
			return fmt.Errorf("generate id: %w", err)
		}
		book.ID = id
	}
	if err := book.Validate(true); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	books, err := s.readAllLocked()
	if err != nil {
		return err
	}

	replaced := false
	for i := range books {
		if books[i].ID == book.ID {
			books[i] = *book
			replaced = true
			break
		}
	}
	if !replaced {
		books = append(books, *book)
	}

	return s.rebuildLocked(books)
}

// Get returns the book with the given id, or ErrBookNotFound. The book is
// reconstructed from its stored fields via IndexSearcher.Doc.
func (s *BookStore) Get(id string) (Book, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	reader, err := index.OpenDirectoryReader(s.dir)
	if err != nil {
		return Book{}, fmt.Errorf("open reader: %w", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	topDocs, err := searcher.Search(search.NewTermQuery(index.NewTerm(FieldID, id)), 1)
	if err != nil {
		return Book{}, fmt.Errorf("lookup by id: %w", err)
	}
	if topDocs == nil || len(topDocs.ScoreDocs) == 0 {
		return Book{}, ErrBookNotFound
	}

	doc, err := searcher.Doc(topDocs.ScoreDocs[0].Doc)
	if err != nil {
		return Book{}, fmt.Errorf("load stored fields: %w", err)
	}
	return bookFromDocument(doc), nil
}

// Delete removes the book with the given id. Returns ErrBookNotFound if no
// such book exists. The live set is read back from the index, the target id
// is dropped, and the index is rebuilt from the survivors.
func (s *BookStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	books, err := s.readAllLocked()
	if err != nil {
		return err
	}

	kept := make([]Book, 0, len(books))
	found := false
	for _, b := range books {
		if b.ID == id {
			found = true
			continue
		}
		kept = append(kept, b)
	}
	if !found {
		return ErrBookNotFound
	}

	return s.rebuildLocked(kept)
}

// readAllLocked reconstructs every live book from the index by running a
// match-all query and hydrating each hit via IndexSearcher.Doc. Results are
// deduplicated by domain id and returned in the reader's natural hit order.
// The caller must hold s.mu.
func (s *BookStore) readAllLocked() ([]Book, error) {
	reader, err := index.OpenDirectoryReader(s.dir)
	if err != nil {
		return nil, fmt.Errorf("open reader: %w", err)
	}
	defer reader.Close()

	num := reader.NumDocs()
	if num == 0 {
		return nil, nil
	}

	searcher := search.NewIndexSearcher(reader)
	topDocs, err := searcher.Search(search.NewMatchAllDocsQuery(), num)
	if err != nil {
		return nil, fmt.Errorf("enumerate documents: %w", err)
	}
	if topDocs == nil {
		return nil, nil
	}

	seen := make(map[string]struct{}, len(topDocs.ScoreDocs))
	books := make([]Book, 0, len(topDocs.ScoreDocs))
	for _, hit := range topDocs.ScoreDocs {
		doc, err := searcher.Doc(hit.Doc)
		if err != nil {
			return nil, fmt.Errorf("load stored fields for doc %d: %w", hit.Doc, err)
		}
		book := bookFromDocument(doc)
		if _, dup := seen[book.ID]; dup {
			continue
		}
		seen[book.ID] = struct{}{}
		books = append(books, book)
	}
	return books, nil
}

// rebuildLocked closes the current writer, wipes the directory, opens a fresh
// writer, re-adds every supplied book, and commits. The caller must hold s.mu
// in write mode.
//
// Rebuilding from scratch (rather than mutating in place via
// UpdateDocument/DeleteDocuments) is required while the Gocene codec read path
// does not apply buffered term-deletes to already-committed segments: an
// in-place update would leave a stale copy visible to a fresh reader and a
// delete would not take effect. A CREATE rebuild produces a single coherent
// segment set with no carried-over deletes, which the reader hydrates exactly.
func (s *BookStore) rebuildLocked(books []Book) error {
	if s.writer != nil {
		if err := s.writer.Close(); err != nil {
			return fmt.Errorf("close writer before rebuild: %w", err)
		}
		s.writer = nil
	}

	entries, err := s.dir.ListAll()
	if err != nil {
		return fmt.Errorf("list directory for rebuild: %w", err)
	}
	for _, name := range entries {
		if err := s.dir.DeleteFile(name); err != nil {
			return fmt.Errorf("delete %q before rebuild: %w", name, err)
		}
	}

	cfg := index.NewIndexWriterConfig(s.analyzer)
	cfg.SetOpenMode(index.CREATE_OR_APPEND)
	writer, err := index.NewIndexWriter(s.dir, cfg)
	if err != nil {
		return fmt.Errorf("open writer for rebuild: %w", err)
	}

	for i := range books {
		doc, err := books[i].toDocument()
		if err != nil {
			_ = writer.Close()
			return fmt.Errorf("build document for %q: %w", books[i].ID, err)
		}
		if err := writer.AddDocument(doc); err != nil {
			_ = writer.Close()
			return fmt.Errorf("index %q: %w", books[i].ID, err)
		}
	}
	if err := writer.Commit(); err != nil {
		_ = writer.Close()
		return fmt.Errorf("commit: %w", err)
	}
	s.writer = writer
	return nil
}

// SearchRequest captures the parameters accepted by the search endpoint.
type SearchRequest struct {
	Query string
	Field string
	Page  int
	Size  int
}

// SearchResult is the paginated payload returned by Search.
type SearchResult struct {
	Total int    `json:"total"`
	Page  int    `json:"page"`
	Size  int    `json:"size"`
	Items []Book `json:"items"`
}

// Search runs a paginated query against the live index. When req.Field is
// empty the query is parsed against the default full-text fields; when it
// names a specific field the query is restricted to that field only. The
// exact-match fields (id, year) are non-tokenised StringFields, so they are
// resolved with a direct TermQuery rather than through the analyzer-backed
// query parser (the parser rejects a bare year such as "1999" and would
// tokenise an id). Every hit is hydrated from its stored fields via
// IndexSearcher.Doc and deduplicated by domain id. Pagination is naive: the
// searcher is asked for a generous slice and the requested page is sliced out
// — fine for a demo corpus.
//
// Errors are classified so callers can map them to HTTP status codes:
//   - ErrUnknownField → 400 (bad field parameter)
//   - ErrBadQuery → 400 (unparseable query string)
//   - all other errors → 500 (internal / I/O failures)
func (s *BookStore) Search(req SearchRequest) (SearchResult, error) {
	page, size := normalisePaging(req.Page, req.Size)

	if req.Field != "" && !IsValidField(req.Field) {
		return SearchResult{}, fmt.Errorf("%w: %q", ErrUnknownField, req.Field)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	reader, err := index.OpenDirectoryReader(s.dir)
	if err != nil {
		return SearchResult{}, fmt.Errorf("open reader: %w", err)
	}
	defer reader.Close()

	num := reader.NumDocs()
	if num == 0 {
		return SearchResult{Page: page, Size: size, Items: []Book{}}, nil
	}

	searcher := search.NewIndexSearcher(reader)

	query, err := s.buildQuery(req)
	if err != nil {
		return SearchResult{}, fmt.Errorf("%w: %w", ErrBadQuery, err)
	}

	// Ask the searcher for every live document. A TermQuery hit is one per
	// matching document, but we still deduplicate defensively by domain id so
	// the page-to-page invariant (no id appears on two pages) holds regardless
	// of how the scorer enumerates multi-valued fields.
	limit := num
	if limit < size {
		limit = size
	}
	topDocs, err := searcher.Search(query, limit)
	if err != nil {
		return SearchResult{}, fmt.Errorf("search: %w", err)
	}
	if topDocs == nil {
		return SearchResult{Total: 0, Page: page, Size: size, Items: []Book{}}, nil
	}

	seen := make(map[string]struct{}, len(topDocs.ScoreDocs))
	unique := make([]Book, 0, len(topDocs.ScoreDocs))
	for _, hit := range topDocs.ScoreDocs {
		doc, err := searcher.Doc(hit.Doc)
		if err != nil {
			return SearchResult{}, fmt.Errorf("load stored fields for doc %d: %w", hit.Doc, err)
		}
		book := bookFromDocument(doc)
		if _, dup := seen[book.ID]; dup {
			continue
		}
		seen[book.ID] = struct{}{}
		unique = append(unique, book)
	}

	total := len(unique)
	start := (page - 1) * size
	if start >= total {
		return SearchResult{Total: total, Page: page, Size: size, Items: []Book{}}, nil
	}
	end := start + size
	if end > total {
		end = total
	}
	return SearchResult{Total: total, Page: page, Size: size, Items: unique[start:end]}, nil
}

// buildQuery turns a SearchRequest into a Gocene Query. The exact-match fields
// (id, year) bypass the query parser and use a direct TermQuery so a bare
// value matches the non-tokenised StringField verbatim. An empty query string
// matches everything.
func (s *BookStore) buildQuery(req SearchRequest) (search.Query, error) {
	if req.Field == FieldID || req.Field == FieldYear {
		if req.Query == "" {
			return search.NewMatchAllDocsQuery(), nil
		}
		return search.NewTermQuery(index.NewTerm(req.Field, req.Query)), nil
	}

	q := req.Query
	if q == "" {
		return search.NewMatchAllDocsQuery(), nil
	}

	if req.Field == "" {
		parser := queryparser.NewMultiFieldQueryParser(SearchableFields, s.analyzer)
		query, err := parser.Parse(q)
		if err != nil {
			return nil, fmt.Errorf("parse multi-field query: %w", err)
		}
		return query, nil
	}

	parser := queryparser.NewQueryParser(req.Field, s.analyzer)
	query, err := parser.Parse(q)
	if err != nil {
		return nil, fmt.Errorf("parse %s query: %w", req.Field, err)
	}
	return query, nil
}

func normalisePaging(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	if size > 100 {
		size = 100
	}
	return page, size
}

func generateID() (string, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return "book-" + hex.EncodeToString(buf[:]), nil
}

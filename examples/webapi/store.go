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
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queryparser"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ErrBookNotFound is returned by Get/Update/Delete when no document with the
// requested id is indexed.
var ErrBookNotFound = errors.New("book not found")

// BookStore exposes a small CRUD + search surface tailored to the Book domain
// and backed by a Gocene index for full-text matching.
//
// In the current state of Gocene the read path that goes through
// IndexSearcher.Doc(docID) is not yet operational — see README.md ("Known
// limitations") and rmp task 4636 for the underlying gap. To keep the demo
// fully functional this store also maintains an in-memory shadow of every
// book and remaps internal doc IDs back to domain ids through a dedicated
// order slice. The index drives discovery (queries, scoring, pagination);
// the shadow drives round-trip retrieval.
type BookStore struct {
	dir      *store.MMapDirectory
	analyzer *analysis.StandardAnalyzer

	mu     sync.RWMutex
	writer *index.IndexWriter
	books  map[string]Book // domain id -> Book; source of truth for reads
	order  []string        // domain ids in insertion order; position == index doc id after rebuild
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
		books:    make(map[string]Book),
		order:    nil,
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

// IsEmpty reports whether the store currently holds zero books.
func (s *BookStore) IsEmpty() (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.books) == 0, nil
}

// Put indexes a new book. If book.ID is empty an id is generated and assigned
// in place. Existing ids are replaced.
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

	if _, existed := s.books[book.ID]; !existed {
		s.order = append(s.order, book.ID)
	}
	s.books[book.ID] = *book

	return s.rebuildIndexLocked()
}

// Get returns the book with the given id, or ErrBookNotFound.
func (s *BookStore) Get(id string) (Book, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	book, ok := s.books[id]
	if !ok {
		return Book{}, ErrBookNotFound
	}
	return book, nil
}

// Delete removes the book with the given id. Returns ErrBookNotFound if no
// such book exists.
func (s *BookStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.books[id]; !ok {
		return ErrBookNotFound
	}
	delete(s.books, id)
	s.order = removeString(s.order, id)

	return s.rebuildIndexLocked()
}

// rebuildIndexLocked closes the current writer, opens a fresh writer in
// CREATE mode, re-adds every book in insertion order, and commits.
// After this call the internal doc id assigned by Gocene for the book at
// s.order[k] is exactly k, which is what Search relies on to map hits back
// to domain ids. The caller must hold s.mu in write mode.
//
// Closing and reopening the writer on every rebuild avoids a state mismatch
// in the directory where DeleteAll + AddDocument + Commit leaves stale
// segment data that causes field-level term queries to return wrong results.
func (s *BookStore) rebuildIndexLocked() error {
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

	for _, id := range s.order {
		book := s.books[id]
		doc, err := book.toDocument()
		if err != nil {
			_ = writer.Close()
			return fmt.Errorf("build document for %q: %w", id, err)
		}
		if err := writer.AddDocument(doc); err != nil {
			_ = writer.Close()
			return fmt.Errorf("index %q: %w", id, err)
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

// Search runs a paginated query. When req.Field is empty the query is parsed
// against the default full-text fields; when it names a specific field the
// query is restricted to that field only. Exact-match fields (id, year) are
// resolved directly against the in-memory shadow rather than the Gocene
// index — the index's term-level scoping for non-tokenised StringField is
// still being firmed up upstream (see README "Known limitations"), so the
// shadow path gives the demo a deterministic answer. Pagination is naive:
// the searcher is asked for the first page*size hits and the requested page
// is sliced out — fine for a demo corpus.
func (s *BookStore) Search(req SearchRequest) (SearchResult, error) {
	page, size := normalisePaging(req.Page, req.Size)

	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.books) == 0 {
		return SearchResult{Page: page, Size: size, Items: []Book{}}, nil
	}

	if req.Field == FieldID || req.Field == FieldYear {
		return s.exactMatchSearchLocked(req, page, size)
	}

	if !IsValidField(req.Field) && req.Field != "" {
		return SearchResult{}, fmt.Errorf("unknown field %q", req.Field)
	}

	reader, err := index.OpenDirectoryReader(s.dir)
	if err != nil {
		return SearchResult{}, fmt.Errorf("open reader: %w", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	query, err := s.buildQuery(req)
	if err != nil {
		return SearchResult{}, err
	}

	// Ask the searcher for a generous slice. The hit count Gocene returns
	// today over-counts multi-valued fields (one hit per (doc, value) pair),
	// so we deduplicate by domain id and use the deduped length as the true
	// total.
	limit := len(s.order) * 16
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
		if hit.Doc < 0 || hit.Doc >= len(s.order) {
			continue
		}
		id := s.order[hit.Doc]
		if _, dup := seen[id]; dup {
			continue
		}
		book, ok := s.books[id]
		if !ok {
			continue
		}
		seen[id] = struct{}{}
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

func (s *BookStore) buildQuery(req SearchRequest) (search.Query, error) {
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

// exactMatchSearchLocked answers id= and year= queries directly from the
// shadow map. The caller must hold s.mu.
func (s *BookStore) exactMatchSearchLocked(req SearchRequest, page, size int) (SearchResult, error) {
	matches := make([]Book, 0)

	switch req.Field {
	case FieldID:
		if book, ok := s.books[req.Query]; ok {
			matches = append(matches, book)
		}
	case FieldYear:
		for _, id := range s.order {
			book := s.books[id]
			if fmt.Sprintf("%d", book.Year) == req.Query {
				matches = append(matches, book)
			}
		}
	}

	total := len(matches)
	start := (page - 1) * size
	if start >= total {
		return SearchResult{Total: total, Page: page, Size: size, Items: []Book{}}, nil
	}
	end := start + size
	if end > total {
		end = total
	}
	return SearchResult{Total: total, Page: page, Size: size, Items: matches[start:end]}, nil
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

func removeString(s []string, target string) []string {
	for i, v := range s {
		if v == target {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

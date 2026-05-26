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

// BookStore wraps a Gocene index and exposes a small CRUD + search surface
// tailored to the Book domain. It is safe for concurrent use: writes are
// serialised through a mutex, and each read opens a fresh DirectoryReader so
// it observes the latest committed snapshot without contending with writes.
type BookStore struct {
	dir      *store.MMapDirectory
	analyzer *analysis.StandardAnalyzer

	mu     sync.Mutex
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

// IsEmpty reports whether the index currently holds zero documents. Used by
// the seeding logic to populate the golden corpus on a fresh boot.
func (s *BookStore) IsEmpty() (bool, error) {
	reader, err := index.OpenDirectoryReader(s.dir)
	if err != nil {
		// A fresh directory with no committed segments is treated as empty.
		return true, nil
	}
	defer reader.Close()
	return reader.NumDocs() == 0, nil
}

// Put indexes a new book. If book.ID is empty an id is generated and assigned
// in place. UpdateDocument is used so re-indexing an existing id replaces the
// previous document instead of duplicating it.
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

	doc, err := book.toDocument()
	if err != nil {
		return fmt.Errorf("build document: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	term := index.NewTerm(FieldID, book.ID)
	if err := s.writer.UpdateDocument(term, doc); err != nil {
		return fmt.Errorf("index document: %w", err)
	}
	if err := s.writer.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// Get returns the book with the given id, or ErrBookNotFound.
func (s *BookStore) Get(id string) (Book, error) {
	reader, err := index.OpenDirectoryReader(s.dir)
	if err != nil {
		return Book{}, ErrBookNotFound
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewTermQuery(index.NewTerm(FieldID, id))

	topDocs, err := searcher.Search(query, 1)
	if err != nil {
		return Book{}, fmt.Errorf("search by id: %w", err)
	}
	if topDocs == nil || len(topDocs.ScoreDocs) == 0 {
		return Book{}, ErrBookNotFound
	}

	doc, err := searcher.Doc(topDocs.ScoreDocs[0].Doc)
	if err != nil {
		return Book{}, fmt.Errorf("load document: %w", err)
	}
	return bookFromDocument(doc), nil
}

// Delete removes the book with the given id. It returns ErrBookNotFound if no
// such document is indexed.
func (s *BookStore) Delete(id string) error {
	if _, err := s.Get(id); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.writer.DeleteDocuments(index.NewTerm(FieldID, id)); err != nil {
		return fmt.Errorf("delete document: %w", err)
	}
	if err := s.writer.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
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
// query is restricted to that field only. Pagination is naive: the searcher
// is asked for the first page*size hits and the requested page is sliced out
// — this keeps the example free of NRT/collector wiring while preserving the
// observable behaviour callers expect from a paginated API.
func (s *BookStore) Search(req SearchRequest) (SearchResult, error) {
	page, size := normalisePaging(req.Page, req.Size)

	reader, err := index.OpenDirectoryReader(s.dir)
	if err != nil {
		return SearchResult{Page: page, Size: size}, nil
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	query, err := s.buildQuery(req)
	if err != nil {
		return SearchResult{}, err
	}

	limit := page * size
	topDocs, err := searcher.Search(query, limit)
	if err != nil {
		return SearchResult{}, fmt.Errorf("search: %w", err)
	}
	if topDocs == nil {
		return SearchResult{Page: page, Size: size}, nil
	}

	hits := topDocs.ScoreDocs
	total := int(topDocs.TotalHits.Value)
	start := (page - 1) * size
	if start >= len(hits) {
		return SearchResult{Total: total, Page: page, Size: size, Items: []Book{}}, nil
	}
	end := start + size
	if end > len(hits) {
		end = len(hits)
	}

	items := make([]Book, 0, end-start)
	for _, hit := range hits[start:end] {
		doc, err := searcher.Doc(hit.Doc)
		if err != nil {
			return SearchResult{}, fmt.Errorf("load document %d: %w", hit.Doc, err)
		}
		items = append(items, bookFromDocument(doc))
	}

	return SearchResult{Total: total, Page: page, Size: size, Items: items}, nil
}

func (s *BookStore) buildQuery(req SearchRequest) (search.Query, error) {
	q := req.Query
	if q == "" {
		return search.NewMatchAllDocsQuery(), nil
	}

	switch req.Field {
	case "":
		parser := queryparser.NewMultiFieldQueryParser(SearchableFields, s.analyzer)
		query, err := parser.Parse(q)
		if err != nil {
			return nil, fmt.Errorf("parse multi-field query: %w", err)
		}
		return query, nil
	case FieldID, FieldYear:
		// Exact-match fields: side-step the analyzer so callers can match the
		// raw stored term verbatim (e.g. field=year&q=1999).
		return search.NewTermQuery(index.NewTerm(req.Field, q)), nil
	default:
		if !IsValidField(req.Field) {
			return nil, fmt.Errorf("unknown field %q", req.Field)
		}
		parser := queryparser.NewQueryParser(req.Field, s.analyzer)
		query, err := parser.Parse(q)
		if err != nil {
			return nil, fmt.Errorf("parse %s query: %w", req.Field, err)
		}
		return query, nil
	}
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

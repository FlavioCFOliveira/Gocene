// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package webapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// maxRequestBodySize caps the number of bytes read from a request body before
// JSON decoding. Bodies larger than this are rejected with 413 Request Entity
// Too Large, preventing memory exhaustion from oversized payloads. 1 MiB is
// generous for a single Book document while still bounding per-request memory.
const maxRequestBodySize = 1 << 20 // 1 MiB

// genericBadRequestMessage is returned to clients when a request body cannot be
// decoded. The concrete decode error is logged server-side rather than echoed
// back, so internal details (file paths, parser internals) never leak.
const genericBadRequestMessage = "invalid request body"

// genericServerErrorMessage is returned to clients for any unexpected
// server-side failure. The concrete error is logged server-side only.
const genericServerErrorMessage = "internal server error"

// Server exposes the HTTP routes that drive the BookStore. It implements
// http.Handler so it can be mounted directly into net/http servers.
type Server struct {
	store  *BookStore
	mux    *http.ServeMux
	logger *slog.Logger
}

// NewServer builds a Server bound to the given BookStore, using the default
// slog logger for server-side error reporting. Routes:
//
//	POST   /books          create
//	GET    /books          paginated search (q, field, page, size)
//	GET    /books/{id}     read
//	PUT    /books/{id}     update
//	DELETE /books/{id}     delete
//	GET    /healthz        liveness probe
func NewServer(s *BookStore) *Server {
	return NewServerWithLogger(s, slog.Default())
}

// NewServerWithLogger builds a Server bound to the given BookStore and logger.
// The logger receives the concrete internal errors that are deliberately kept
// out of HTTP responses. A nil logger falls back to slog.Default().
func NewServerWithLogger(s *BookStore, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	srv := &Server{store: s, mux: http.NewServeMux(), logger: logger}
	srv.mux.HandleFunc("/healthz", srv.handleHealth)
	srv.mux.HandleFunc("/books", srv.handleBooksCollection)
	srv.mux.HandleFunc("/books/", srv.handleBookItem)
	return srv
}

// ServeHTTP routes the request to the appropriate handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleBooksCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.searchBooks(w, r)
	case http.MethodPost:
		s.createBook(w, r)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleBookItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/books/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getBook(w, r, id)
	case http.MethodPut:
		s.updateBook(w, r, id)
	case http.MethodDelete:
		s.deleteBook(w, r, id)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodDelete)
	}
}

func (s *Server) createBook(w http.ResponseWriter, r *http.Request) {
	var book Book
	if !s.decodeBody(w, r, &book) {
		return
	}
	if err := book.Validate(false); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.Put(&book); err != nil {
		s.serverError(w, r, "put book", err)
		return
	}
	w.Header().Set("Location", "/books/"+book.ID)
	writeJSON(w, http.StatusCreated, book)
}

func (s *Server) getBook(w http.ResponseWriter, r *http.Request, id string) {
	book, err := s.store.Get(id)
	if err != nil {
		if errors.Is(err, ErrBookNotFound) {
			writeError(w, http.StatusNotFound, "book not found")
			return
		}
		s.serverError(w, r, "get book", err)
		return
	}
	writeJSON(w, http.StatusOK, book)
}

func (s *Server) updateBook(w http.ResponseWriter, r *http.Request, id string) {
	// PUT is update-only; reject requests for ids that do not exist.
	if _, err := s.store.Get(id); err != nil {
		if errors.Is(err, ErrBookNotFound) {
			writeError(w, http.StatusNotFound, "book not found")
			return
		}
		s.serverError(w, r, "get book for update", err)
		return
	}

	var book Book
	if !s.decodeBody(w, r, &book) {
		return
	}
	if book.ID != "" && book.ID != id {
		writeError(w, http.StatusBadRequest, "id in body does not match URL")
		return
	}
	book.ID = id
	if err := book.Validate(true); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.Put(&book); err != nil {
		s.serverError(w, r, "put book", err)
		return
	}
	writeJSON(w, http.StatusOK, book)
}

func (s *Server) deleteBook(w http.ResponseWriter, r *http.Request, id string) {
	if err := s.store.Delete(id); err != nil {
		if errors.Is(err, ErrBookNotFound) {
			writeError(w, http.StatusNotFound, "book not found")
			return
		}
		s.serverError(w, r, "delete book", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) searchBooks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := SearchRequest{
		Query: q.Get("q"),
		Field: q.Get("field"),
	}
	if v := q.Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "page must be a positive integer")
			return
		}
		req.Page = n
	}
	if v := q.Get("size"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "size must be a positive integer")
			return
		}
		req.Size = n
	}

	result, err := s.store.Search(req)
	if err != nil {
		switch {
		case errors.Is(err, ErrUnknownField):
			// The sentinel's own message is user-safe; the wrapped form may
			// carry parser/internal detail, so log it but do not echo it.
			s.logger.WarnContext(r.Context(), "search rejected", slog.String("reason", "unknown field"), slog.Any("error", err))
			writeError(w, http.StatusBadRequest, ErrUnknownField.Error())
		case errors.Is(err, ErrBadQuery):
			s.logger.WarnContext(r.Context(), "search rejected", slog.String("reason", "bad query"), slog.Any("error", err))
			writeError(w, http.StatusBadRequest, ErrBadQuery.Error())
		default:
			s.serverError(w, r, "search", err)
		}
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// decodeBody validates the request Content-Type, bounds the body to
// maxRequestBodySize, and decodes the JSON payload into dst. It writes the
// appropriate error response itself and returns false when the request must be
// rejected; callers should simply return on a false result. Internal decode
// errors are logged server-side and never echoed to the client.
//
// Status codes:
//   - 415 Unsupported Media Type: Content-Type is missing or not
//     application/json.
//   - 413 Request Entity Too Large: the body exceeds maxRequestBodySize.
//   - 400 Bad Request: the body is malformed, empty, or carries unknown fields.
func (s *Server) decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	if !hasJSONContentType(r) {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return false
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		s.writeDecodeError(w, r, err)
		return false
	}

	// Reject trailing data after the first JSON value. Without this check a
	// tiny valid object followed by a large junk tail would decode cleanly and
	// slip past the body-size guard (the first Decode stops at the first
	// value, never touching the tail). Requiring io.EOF closes that gap and
	// also rejects accidental multi-document bodies.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("unexpected trailing data after JSON value")
		}
		s.writeDecodeError(w, r, err)
		return false
	}
	return true
}

// writeDecodeError classifies a body-decode failure into a 413 (body exceeded
// maxRequestBodySize) or a generic 400, logging the concrete error server-side
// without echoing it to the client.
func (s *Server) writeDecodeError(w http.ResponseWriter, r *http.Request, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		s.logger.WarnContext(r.Context(), "request body too large",
			slog.Int64("limit_bytes", maxRequestBodySize), slog.Any("error", err))
		writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
		return
	}
	s.logger.WarnContext(r.Context(), "request body decode failed", slog.Any("error", err))
	writeError(w, http.StatusBadRequest, genericBadRequestMessage)
}

// hasJSONContentType reports whether the request declares a JSON body. The
// media-type token is matched case-insensitively and any parameters (e.g.
// "; charset=utf-8") are ignored, per RFC 9110.
func hasJSONContentType(r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		return false
	}
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.EqualFold(strings.TrimSpace(ct), "application/json")
}

// serverError logs the concrete internal error against the request context and
// returns a generic 500 response that leaks no internal detail.
func (s *Server) serverError(w http.ResponseWriter, r *http.Request, op string, err error) {
	s.logger.ErrorContext(r.Context(), "request failed",
		slog.String("op", op),
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.Any("error", err))
	writeError(w, http.StatusInternalServerError, genericServerErrorMessage)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func methodNotAllowed(w http.ResponseWriter, allowed ...string) {
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

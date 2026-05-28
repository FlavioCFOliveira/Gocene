// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package webapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// Server exposes the HTTP routes that drive the BookStore. It implements
// http.Handler so it can be mounted directly into net/http servers.
type Server struct {
	store *BookStore
	mux   *http.ServeMux
}

// NewServer builds a Server bound to the given BookStore. Routes:
//
//	POST   /books          create
//	GET    /books          paginated search (q, field, page, size)
//	GET    /books/{id}     read
//	PUT    /books/{id}     update
//	DELETE /books/{id}     delete
//	GET    /healthz        liveness probe
func NewServer(s *BookStore) *Server {
	srv := &Server{store: s, mux: http.NewServeMux()}
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
	if err := decodeJSON(r, &book); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := book.Validate(false); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.Put(&book); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Location", "/books/"+book.ID)
	writeJSON(w, http.StatusCreated, book)
}

func (s *Server) getBook(w http.ResponseWriter, _ *http.Request, id string) {
	book, err := s.store.Get(id)
	if err != nil {
		if errors.Is(err, ErrBookNotFound) {
			writeError(w, http.StatusNotFound, "book not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
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
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var book Book
	if err := decodeJSON(r, &book); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, book)
}

func (s *Server) deleteBook(w http.ResponseWriter, _ *http.Request, id string) {
	if err := s.store.Delete(id); err != nil {
		if errors.Is(err, ErrBookNotFound) {
			writeError(w, http.StatusNotFound, "book not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
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
		if errors.Is(err, ErrUnknownField) || errors.Is(err, ErrBadQuery) {
			writeError(w, http.StatusBadRequest, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	return nil
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

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package webapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/examples/webapi"
)

// TestWebAPI_CRUDLifecycle drives the full HTTP lifecycle of the example:
// seeding from the embedded golden corpus, creating, reading, updating,
// paginated searching across different fields, and deleting a Book.
//
// It satisfies acceptance criterion #3 of sprint 115 / task 4635.
func TestWebAPI_CRUDLifecycle(t *testing.T) {
	t.Parallel()

	srv, baseURL, cleanup := startServer(t)
	defer cleanup()
	_ = srv

	// --- seed corpus is in place ---------------------------------------------
	got := doJSONRequest[webapi.SearchResult](t, http.MethodGet, baseURL+"/books?size=50", nil)
	if got.Total < 10 {
		t.Fatalf("seed corpus too small: got total=%d (want >=10)", got.Total)
	}
	seededTotal := got.Total

	// --- POST creates a new book --------------------------------------------
	newBook := webapi.Book{
		Title:   "Gocene in Action",
		Author:  "Flavio Oliveira",
		Year:    2026,
		Tags:    []string{"gocene", "lucene", "go"},
		Summary: "Hands-on guide to using the Gocene module to build search applications in Go.",
	}
	created := doJSONRequestExpect[webapi.Book](t, http.MethodPost, baseURL+"/books", newBook, http.StatusCreated)
	if created.ID == "" {
		t.Fatalf("created book has empty id")
	}
	if created.Title != newBook.Title {
		t.Fatalf("created title = %q, want %q", created.Title, newBook.Title)
	}

	// --- GET /books/{id} returns the new book --------------------------------
	fetched := doJSONRequest[webapi.Book](t, http.MethodGet, baseURL+"/books/"+created.ID, nil)
	if fetched.ID != created.ID || fetched.Author != "Flavio Oliveira" || fetched.Year != 2026 {
		t.Fatalf("fetched book mismatch: %+v", fetched)
	}
	if len(fetched.Tags) != 3 {
		t.Fatalf("expected 3 tags, got %d (%v)", len(fetched.Tags), fetched.Tags)
	}

	// --- PUT updates the book ------------------------------------------------
	updated := fetched
	updated.Summary = "Revised hands-on guide covering the Gocene module."
	updated.Tags = append(updated.Tags, "revised")
	doJSONRequestExpect[webapi.Book](t, http.MethodPut, baseURL+"/books/"+created.ID, updated, http.StatusOK)

	refetched := doJSONRequest[webapi.Book](t, http.MethodGet, baseURL+"/books/"+created.ID, nil)
	if !strings.Contains(refetched.Summary, "Revised") {
		t.Fatalf("update not observed: summary=%q", refetched.Summary)
	}
	if len(refetched.Tags) != 4 {
		t.Fatalf("expected 4 tags after update, got %d (%v)", len(refetched.Tags), refetched.Tags)
	}

	// --- GET /books?q=... paginates and filters by field --------------------
	allCraft := doJSONRequest[webapi.SearchResult](t, http.MethodGet,
		baseURL+"/books?q=craftsmanship&field=tags&size=10", nil)
	if allCraft.Total < 2 {
		t.Fatalf("expected at least 2 craftsmanship hits, got %d", allCraft.Total)
	}

	titleGo := doJSONRequest[webapi.SearchResult](t, http.MethodGet,
		baseURL+"/books?q=programming&field=title&page=1&size=5", nil)
	if titleGo.Size != 5 || titleGo.Page != 1 {
		t.Fatalf("pagination metadata wrong: %+v", titleGo)
	}
	if len(titleGo.Items) == 0 {
		t.Fatalf("expected at least one hit for q=programming field=title, got 0 (total=%d)", titleGo.Total)
	}

	// Pagination invariant: second page must not duplicate the first page's ids.
	if titleGo.Total > len(titleGo.Items) {
		page2 := doJSONRequest[webapi.SearchResult](t, http.MethodGet,
			baseURL+"/books?q=programming&field=title&page=2&size=5", nil)
		seen := map[string]struct{}{}
		for _, b := range titleGo.Items {
			seen[b.ID] = struct{}{}
		}
		for _, b := range page2.Items {
			if _, dup := seen[b.ID]; dup {
				t.Fatalf("page 2 returned duplicate id %q from page 1", b.ID)
			}
		}
	}

	// Exact-match field: year=1999.
	yr := doJSONRequest[webapi.SearchResult](t, http.MethodGet,
		baseURL+"/books?q=1999&field=year&size=20", nil)
	if yr.Total < 1 {
		t.Fatalf("expected at least one 1999 book, got %d", yr.Total)
	}
	for _, b := range yr.Items {
		if b.Year != 1999 {
			t.Fatalf("year filter leaked non-1999 book: %+v", b)
		}
	}

	// --- DELETE removes the new book ----------------------------------------
	status := doRequestExpectStatus(t, http.MethodDelete, baseURL+"/books/"+created.ID, nil, http.StatusNoContent)
	if status != http.StatusNoContent {
		t.Fatalf("DELETE returned %d, want 204", status)
	}

	// Subsequent GET must 404.
	doRequestExpectStatus(t, http.MethodGet, baseURL+"/books/"+created.ID, nil, http.StatusNotFound)

	// Total count returns to the seed baseline.
	finalCount := doJSONRequest[webapi.SearchResult](t, http.MethodGet, baseURL+"/books?size=100", nil)
	if finalCount.Total != seededTotal {
		t.Fatalf("after delete, total=%d (want seed baseline %d)", finalCount.Total, seededTotal)
	}
}

// TestWebAPI_RejectsBadInput exercises a handful of error paths.
func TestWebAPI_RejectsBadInput(t *testing.T) {
	t.Parallel()

	_, baseURL, cleanup := startServer(t)
	defer cleanup()

	doRequestExpectStatus(t, http.MethodPost, baseURL+"/books",
		webapi.Book{Title: ""}, http.StatusBadRequest)
	doRequestExpectStatus(t, http.MethodGet, baseURL+"/books?field=nope&q=x",
		nil, http.StatusBadRequest)
	doRequestExpectStatus(t, http.MethodGet, baseURL+"/books/does-not-exist",
		nil, http.StatusNotFound)
	doRequestExpectStatus(t, http.MethodPatch, baseURL+"/books",
		nil, http.StatusMethodNotAllowed)
}

// startServer spins up the Gocene-backed Server on an httptest.Server. The
// returned cleanup function closes both the test server and the underlying
// BookStore.
func startServer(t *testing.T) (*webapi.Server, string, func()) {
	t.Helper()

	dataDir := filepath.Join(t.TempDir(), "index")
	store, err := webapi.OpenBookStore(dataDir)
	if err != nil {
		t.Fatalf("open book store: %v", err)
	}
	if _, err := webapi.SeedIfEmpty(store); err != nil {
		_ = store.Close()
		t.Fatalf("seed corpus: %v", err)
	}

	srv := webapi.NewServer(store)
	ts := httptest.NewServer(srv)
	cleanup := func() {
		ts.Close()
		if err := store.Close(); err != nil {
			t.Logf("close store: %v", err)
		}
	}
	return srv, ts.URL, cleanup
}

func doJSONRequest[T any](t *testing.T, method, url string, body any) T {
	t.Helper()
	return doJSONRequestExpect[T](t, method, url, body, http.StatusOK)
}

func doJSONRequestExpect[T any](t *testing.T, method, url string, body any, want int) T {
	t.Helper()

	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != want {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s %s: status=%d (want %d), body=%s", method, url, resp.StatusCode, want, raw)
	}
	var out T
	if resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
	return out
}

func doRequestExpectStatus(t *testing.T, method, url string, body any, want int) int {
	t.Helper()

	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != want {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s %s: status=%d (want %d), body=%s", method, url, resp.StatusCode, want, raw)
	}
	return resp.StatusCode
}

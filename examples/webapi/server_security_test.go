// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package webapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/examples/webapi"
)

// newTestServer builds a Server backed by a seeded store and a logger that
// captures every record into buf, so tests can assert that the concrete
// internal error is logged server-side even when it is withheld from the
// HTTP response. The returned cleanup tears the store down.
func newTestServer(t *testing.T, buf *bytes.Buffer) (*httptest.Server, func()) {
	t.Helper()

	dataDir := t.TempDir()
	store, err := webapi.OpenBookStore(dataDir)
	if err != nil {
		t.Fatalf("open book store: %v", err)
	}
	if _, err := webapi.SeedIfEmpty(store); err != nil {
		_ = store.Close()
		t.Fatalf("seed corpus: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ts := httptest.NewServer(webapi.NewServerWithLogger(store, logger))
	cleanup := func() {
		ts.Close()
		if err := store.Close(); err != nil {
			t.Logf("close store: %v", err)
		}
	}
	return ts, cleanup
}

// firstSeededID returns the id of the first book in the seeded corpus, so
// tests that need an existing resource do not hard-code a generated id.
func firstSeededID(t *testing.T, baseURL string) string {
	t.Helper()
	resp, err := http.Get(baseURL + "/books?size=1")
	if err != nil {
		t.Fatalf("list books: %v", err)
	}
	defer resp.Body.Close()
	var result webapi.SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode search result: %v", err)
	}
	if len(result.Items) == 0 {
		t.Fatalf("seed corpus is empty; cannot obtain an existing id")
	}
	return result.Items[0].ID
}

// errorBody decodes the {"error": "..."} envelope written by writeError.
func errorBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	return payload.Error
}

// TestWebAPI_OversizedBodyRejected asserts that a request body exceeding the
// 1 MiB cap is rejected with 413 and that the limit is enforced on both POST
// and PUT decode paths.
func TestWebAPI_OversizedBodyRejected(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	ts, cleanup := newTestServer(t, &logs)
	defer cleanup()

	// A single valid Book whose summary field carries > 1 MiB of text. This is
	// a well-formed JSON document, so the only reason it is rejected is the
	// body-size guard fired by http.MaxBytesReader during decode.
	huge := webapi.Book{
		Title:   "x",
		Author:  "y",
		Year:    2026,
		Summary: strings.Repeat("A", 2<<20), // 2 MiB
	}
	payload, err := json.Marshal(huge)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if len(payload) <= 1<<20 {
		t.Fatalf("test payload not large enough: %d bytes", len(payload))
	}

	// PUT decodes the body only for an existing id (it 404s before reading the
	// body otherwise — by design, so a huge body for a missing resource is
	// never consumed). Fetch a real seeded id so the PUT path reaches decode.
	existingID := firstSeededID(t, ts.URL)

	for _, method := range []string{http.MethodPost, http.MethodPut} {
		url := ts.URL + "/books"
		if method == http.MethodPut {
			url = ts.URL + "/books/" + existingID
		}
		req, err := http.NewRequest(method, url, bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("build %s request: %v", method, err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", method, err)
		}

		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			raw, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			t.Fatalf("%s oversized body: status=%d (want 413), body=%s", method, resp.StatusCode, raw)
		}
		msg := errorBody(t, resp)
		resp.Body.Close()
		if strings.Contains(msg, "A") && len(msg) > 64 {
			t.Fatalf("%s 413 response leaked payload content: %q", method, msg)
		}
	}
}

// TestWebAPI_ContentTypeValidation asserts 415 for a missing or non-JSON
// Content-Type on the decode paths, and that a correct Content-Type is
// accepted.
func TestWebAPI_ContentTypeValidation(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	ts, cleanup := newTestServer(t, &logs)
	defer cleanup()

	body := `{"title":"t","author":"a","year":2026,"summary":"this is a sufficiently long summary for validation"}`

	cases := []struct {
		name        string
		contentType string
		want        int
	}{
		{"missing", "", http.StatusUnsupportedMediaType},
		{"text-plain", "text/plain", http.StatusUnsupportedMediaType},
		{"form", "application/x-www-form-urlencoded", http.StatusUnsupportedMediaType},
		{"json", "application/json", http.StatusCreated},
		{"json-with-charset", "application/json; charset=utf-8", http.StatusCreated},
		{"json-uppercase", "APPLICATION/JSON", http.StatusCreated},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, ts.URL+"/books", strings.NewReader(body))
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("do request: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.want {
				raw, _ := io.ReadAll(resp.Body)
				t.Fatalf("content-type %q: status=%d (want %d), body=%s", tc.contentType, resp.StatusCode, tc.want, raw)
			}
		})
	}
}

// TestWebAPI_ErrorResponsesAreSanitised asserts that error responses never
// echo internal detail (file paths, Go error chains, parser internals), while
// the concrete error is still logged server-side.
func TestWebAPI_ErrorResponsesAreSanitised(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	ts, cleanup := newTestServer(t, &logs)
	defer cleanup()

	// Malformed JSON must yield a generic 400 with no decoder internals.
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/books", strings.NewReader(`{"year": "not-a-number"`))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("malformed JSON: status=%d (want 400), body=%s", resp.StatusCode, raw)
	}
	msg := errorBody(t, resp)
	resp.Body.Close()

	assertNoInternalDetail(t, msg)

	// The concrete decode error must have reached the logger.
	if !strings.Contains(logs.String(), "decode failed") {
		t.Fatalf("expected the decode error to be logged server-side; logs=%q", logs.String())
	}
}

// TestWebAPI_TrailingDataRejected asserts that a valid JSON object followed by
// extra data is rejected with 400. Without the trailing-data guard, a tiny
// object with a large junk tail would decode cleanly and bypass the body-size
// limit, because the JSON decoder stops at the first value.
func TestWebAPI_TrailingDataRejected(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	ts, cleanup := newTestServer(t, &logs)
	defer cleanup()

	body := `{"title":"t","author":"a","year":2026,"summary":"a sufficiently long summary text"}{"title":"second"}`
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/books", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("trailing data: status=%d (want 400), body=%s", resp.StatusCode, raw)
	}
	assertNoInternalDetail(t, errorBody(t, resp))
}

// assertNoInternalDetail fails the test if msg looks like it carries internal
// implementation detail rather than a user-safe message.
func assertNoInternalDetail(t *testing.T, msg string) {
	t.Helper()
	forbidden := []string{
		"/",                 // filesystem paths
		".go",               // Go source references
		"github.com",        // module path
		"json:",             // encoding/json error prefixes
		"cannot unmarshal",  // json type errors
		"invalid character", // json syntax errors
		"struct",            // reflection detail
		"0x",                // pointers / offsets
	}
	low := strings.ToLower(msg)
	for _, f := range forbidden {
		if strings.Contains(low, strings.ToLower(f)) {
			t.Fatalf("error response leaked internal detail %q in message %q", f, msg)
		}
	}
}

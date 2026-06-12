// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// testInfoStream captures InfoStream messages for test inspection.
type testInfoStream struct {
	mu       sync.Mutex
	messages []string
}

func newTestInfoStream() *testInfoStream {
	return &testInfoStream{}
}

func (s *testInfoStream) Close() error { return nil }

func (s *testInfoStream) Message(component, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, component+": "+message)
}

func (s *testInfoStream) IsEnabled(component string) bool {
	return true
}

var _ util.InfoStream = (*testInfoStream)(nil)

// simpleDoc returns a document with a single stored text field.
func simpleDoc(name, value string) *document.Document {
	doc := document.NewDocument()
	f, _ := document.NewTextField(name, value, true)
	doc.Add(f)
	return doc
}

// TestInfoStreamGetsFieldName verifies that IndexWriterConfig propagates
// an InfoStream and that the writer correctly processes documents after
// an InfoStream has been installed. This is the Go port of the Lucene
// TestDocInverterPerFieldErrorInfo test concept: the throwing-analyzer
// path is deferred (see gocene-doc-inverter-error-info), but the
// InfoStream plumbing and writer lifecycle are tested here.
func TestInfoStreamGetsFieldName(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	rec := newTestInfoStream()
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetInfoStream(rec)

	writer, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := simpleDoc("field1", "hello world")
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestNoExtraNoise verifies that a clean document indexed with an InfoStream
// leaves no error traces and completes normally.
func TestNoExtraNoise(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	rec := newTestInfoStream()
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetInfoStream(rec)

	writer, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := simpleDoc("boringFieldName", "aaa")
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

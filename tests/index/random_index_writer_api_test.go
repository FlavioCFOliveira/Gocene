// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package index

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// newTestDoc builds a single-field document for the RandomIndexWriter tests.
func newTestDoc(t *testing.T, field, value string) *document.Document {
	t.Helper()
	doc := document.NewDocument()
	f, err := document.NewStringField(field, value, true)
	if err != nil {
		t.Fatalf("NewStringField(%q, %q): %v", field, value, err)
	}
	doc.Add(f)
	return doc
}

func TestRandomIndexWriter_BasicOperations(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	dir := store.NewByteBuffersDirectory()

	riw, err := NewRandomIndexWriter(rng, dir)
	if err != nil {
		t.Fatalf("failed to create RandomIndexWriter: %v", err)
	}
	defer func() { _ = riw.Close() }()

	seqNo, err := riw.AddDocument(newTestDoc(t, "text", "hello world"))
	if err != nil {
		t.Fatalf("AddDocument failed: %v", err)
	}
	if seqNo <= 0 {
		t.Errorf("AddDocument: expected positive seqNo, got %d", seqNo)
	}

	docs := []*document.Document{
		newTestDoc(t, "text", "doc 2"),
		newTestDoc(t, "text", "doc 3"),
	}
	if seqNo, err = riw.AddDocuments(docs); err != nil {
		t.Fatalf("AddDocuments failed: %v", err)
	}
	if seqNo <= 0 {
		t.Errorf("AddDocuments: expected positive seqNo, got %d", seqNo)
	}

	if _, err = riw.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	reader, err := riw.GetReader()
	if err != nil {
		t.Fatalf("GetReader failed: %v", err)
	}
	if reader == nil {
		t.Fatal("GetReader returned a nil reader")
	}
	if got := reader.NumDocs(); got != 3 {
		t.Errorf("NumDocs = %d, want 3", got)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("reader Close failed: %v", err)
	}

	if err := riw.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge failed: %v", err)
	}
	if err := riw.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}
}

func TestRandomIndexWriter_RandomConfigs(t *testing.T) {
	for i := 0; i < 10; i++ {
		rng := rand.New(rand.NewSource(int64(i)))
		dir := store.NewByteBuffersDirectory()

		riw, err := NewRandomIndexWriter(rng, dir)
		if err != nil {
			t.Fatalf("iteration %d: failed to create RandomIndexWriter: %v", i, err)
		}

		// Add enough docs to cross flushAt and exercise maybeFlushOrCommit.
		for j := 0; j < 200; j++ {
			if _, err := riw.AddDocument(newTestDoc(t, "text", "random doc")); err != nil {
				t.Fatalf("iteration %d, doc %d: AddDocument failed: %v", i, j, err)
			}
		}

		if err := riw.Close(); err != nil {
			t.Errorf("iteration %d: Close failed: %v", i, err)
		}
	}
}

func TestRandomIndexWriter_UpdateDocument(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	dir := store.NewByteBuffersDirectory()
	riw, err := NewRandomIndexWriter(rng, dir)
	if err != nil {
		t.Fatalf("failed to create RandomIndexWriter: %v", err)
	}
	defer func() { _ = riw.Close() }()

	term := index.NewTerm("id", "1")
	if _, err := riw.AddDocument(newTestDoc(t, "id", "1")); err != nil {
		t.Fatalf("AddDocument failed: %v", err)
	}
	if _, err := riw.UpdateDocument(term, newTestDoc(t, "id", "1")); err != nil {
		t.Fatalf("UpdateDocument failed: %v", err)
	}
}

func TestRandomIndexWriter_MockIndexWriter(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	dir := store.NewByteBuffersDirectory()
	conf := index.NewIndexWriterConfig()

	iw, err := MockIndexWriter(dir, conf, rng)
	if err != nil {
		t.Fatalf("MockIndexWriter failed: %v", err)
	}
	if iw == nil {
		t.Fatal("MockIndexWriter returned nil")
	}
	if err := iw.Close(); err != nil {
		t.Fatalf("IndexWriter Close failed: %v", err)
	}
}

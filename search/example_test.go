// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ExampleIndexSearcher shows how to index a document and then search for it
// using MatchAllDocsQuery.
func ExampleIndexSearcher() {
	// Build an in-memory index with one document.
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		panic(err)
	}
	doc := document.NewDocument()
	tf, err := document.NewTextField("body", "hello gocene", true)
	if err != nil {
		panic(err)
	}
	doc.Add(tf)
	if err := w.AddDocument(doc); err != nil {
		panic(err)
	}
	if err := w.Commit(); err != nil {
		panic(err)
	}
	if err := w.Close(); err != nil {
		panic(err)
	}

	// Open reader and searcher.
	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		panic(err)
	}
	defer r.Close()

	s := search.NewIndexSearcher(r)
	topDocs, err := s.Search(search.NewMatchAllDocsQuery(), 10)
	if err != nil {
		panic(err)
	}
	fmt.Println(topDocs.TotalHits.Value)
	// Output:
	// 1
}

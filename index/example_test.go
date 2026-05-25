// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ExampleIndexWriter shows how to create an in-memory index, add a document,
// commit, and verify the document count via a DirectoryReader.
func ExampleIndexWriter() {
	// Use an in-memory directory for demonstration.
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Configure and open a writer.
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		panic(err)
	}

	// Build and index one document.
	doc := document.NewDocument()
	tf, err := document.NewTextField("body", "hello gocene world", true)
	if err != nil {
		panic(err)
	}
	doc.Add(tf)
	if err := w.AddDocument(doc); err != nil {
		panic(err)
	}

	// Commit and close the writer.
	if err := w.Commit(); err != nil {
		panic(err)
	}
	if err := w.Close(); err != nil {
		panic(err)
	}

	// Open a reader and check the document count.
	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		panic(err)
	}
	defer r.Close()

	fmt.Println(r.NumDocs())
	// Output:
	// 1
}

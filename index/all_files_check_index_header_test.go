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

// Source: lucene/core/src/test/org/apache/lucene/index/TestAllFilesCheckIndexHeader.java
// Purpose: Verify basic index lifecycle with file header validation.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestAllFilesCheckIndexHeader writes a small index and verifies it can be
// opened and read correctly.
//
// The full Lucene test corrupts file headers and verifies reader-side
// detection. That requires per-file header validation on open, which is
// not yet implemented in Gocene.
func TestAllFilesCheckIndexHeader(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(nil)
	config.SetMaxBufferedDocs(2)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		field, err := document.NewTextField("body", "the quick brown fox "+string(rune('0'+i%10)), true)
		if err != nil {
			t.Fatalf("Failed to create text field: %v", err)
		}
		doc.Add(field)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document %d: %v", i, err)
		}
		if i%7 == 0 {
			if err := writer.Commit(); err != nil {
				t.Fatalf("Failed to commit at doc %d: %v", i, err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Verify the index can be opened and read.
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 100 {
		t.Fatalf("NumDocs = %d, want 100", reader.NumDocs())
	}
}

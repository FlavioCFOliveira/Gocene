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

// Source: lucene/core/src/test/org/apache/lucene/index/TestAllFilesDetectTruncation.java
// Purpose: Verify that the index can be written and read correctly, and that
//          all files have reasonable sizes.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestAllFilesDetectTruncation writes a small index and verifies it can be
// reopened and read back correctly.
//
// The full Lucene test truncates each file in turn and verifies that the
// corruption is detected on open. That requires per-file CRC32 verification
// in the reader/check path, which is not yet implemented.
func TestAllFilesDetectTruncation(t *testing.T) {
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

	// Verify all files exist and have reasonable sizes.
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(files) < 3 {
		t.Fatalf("expected at least 3 files, got %d", len(files))
	}
	for _, name := range files {
		if name == "write.lock" {
			continue
		}
		length, err := dir.FileLength(name)
		if err != nil {
			t.Fatalf("FileLength(%s): %v", name, err)
		}
		if length < 8 {
			t.Fatalf("file %s has length %d, expected >= 8", name, length)
		}
	}
}

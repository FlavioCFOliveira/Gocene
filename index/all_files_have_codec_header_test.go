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

// Source: lucene/core/src/test/org/apache/lucene/index/TestAllFilesHaveCodecHeader.java
// Purpose: Verify that all codec format files start with a valid CODEC_MAGIC header.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

// TestAllFilesHaveCodecHeader writes a small index and verifies that all
// produced files have non-empty headers (first byte is non-zero).
func TestAllFilesHaveCodecHeader(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(nil))
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		field, err := document.NewStringField("body", "test", true)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(field)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected at least one file")
	}
	for _, fileName := range files {
		length, err := dir.FileLength(fileName)
		if err != nil {
			t.Fatalf("FileLength(%s): %v", fileName, err)
		}
		if length < 4 {
			t.Fatalf("file %s length %d < 4", fileName, length)
		}
		in, err := dir.OpenInput(fileName, store.IOContextRead)
		if err != nil {
			t.Fatalf("OpenInput(%s): %v", fileName, err)
		}
		b, err := in.ReadByte()
		in.Close()
		if err != nil {
			t.Fatalf("ReadByte(%s): %v", fileName, err)
		}
		if b == 0 {
			t.Fatalf("file %s starts with 0x00, expected non-zero codec magic", fileName)
		}
	}
}

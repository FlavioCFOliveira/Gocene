// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// GC-143: Integration Tests - Dueling Codecs
// Ported from Apache Lucene's org.apache.lucene.index.TestDuelingCodecs

func TestDuelingCodecs_Basic(t *testing.T) {
	// Use a fixed seed for determinism
	seed := int64(42)

	dir1 := store.NewByteBuffersDirectory()
	defer dir1.Close()

	dir2 := store.NewByteBuffersDirectory()
	defer dir2.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()

	// Index same documents with same configuration
	addRandomDocs(t, dir1, analyzer, seed, 10)
	addRandomDocs(t, dir2, analyzer, seed, 10)

	// Open readers
	reader1, err := index.OpenDirectoryReader(dir1)
	if err != nil {
		t.Fatalf("failed to open reader1: %v", err)
	}
	defer reader1.Close()

	reader2, err := index.OpenDirectoryReader(dir2)
	if err != nil {
		t.Fatalf("failed to open reader2: %v", err)
	}
	defer reader2.Close()

	// Deep comparison
	assertReaderEquals(t, reader1, reader2)
}

func TestDuelingCodecs_Complex(t *testing.T) {
	seed := int64(12345)
	dir1 := store.NewByteBuffersDirectory()
	defer dir1.Close()
	dir2 := store.NewByteBuffersDirectory()
	defer dir2.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()

	// More documents and varied content to exercise segment merging and different field types
	addRandomDocs(t, dir1, analyzer, seed, 100)
	addRandomDocs(t, dir2, analyzer, seed, 100)

	reader1, err := index.OpenDirectoryReader(dir1)
	if err != nil {
		t.Fatalf("failed to open reader1: %v", err)
	}
	defer reader1.Close()

	reader2, err := index.OpenDirectoryReader(dir2)
	if err != nil {
		t.Fatalf("failed to open reader2: %v", err)
	}
	defer reader2.Close()

	assertReaderEquals(t, reader1, reader2)
}

func addRandomDocs(t *testing.T, dir store.Directory, analyzer analysis.Analyzer, seed int64, count int) {
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	r := rand.New(rand.NewSource(seed))

	for i := 0; i < count; i++ {
		doc := document.NewDocument()

		// id field (string)
		idField, _ := document.NewStringField("id", fmt.Sprintf("%d", i), true)
		doc.Add(idField)

		// text field (content)
		contentField, _ := document.NewTextField("content", fmt.Sprintf("random content %d with seed %d", r.Intn(1000), seed), r.Float32() < 0.5)
		doc.Add(contentField)

		// int field
		if r.Float32() < 0.3 {
			age := r.Intn(100)
			ageField, _ := document.NewIntField("age", age, true)
			doc.Add(ageField)
		}

		// stored field
		if r.Float32() < 0.2 {
			metaField, _ := document.NewStoredField("meta", fmt.Sprintf("meta-%d-%d", i, r.Intn(100)))
			doc.Add(metaField)
		}

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add doc %d: %v", i, err)
		}

		// Occasional commit to create segments
		if i > 0 && i%33 == 0 {
			if err := writer.Commit(); err != nil {
				t.Fatalf("failed to commit at %d: %v", i, err)
			}
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
}

func assertReaderEquals(t *testing.T, r1, r2 *index.DirectoryReader) {
	if r1.NumDocs() != r2.NumDocs() {
		t.Errorf("NumDocs mismatch: %d != %d", r1.NumDocs(), r2.NumDocs())
	}
	if r1.MaxDoc() != r2.MaxDoc() {
		t.Errorf("MaxDoc mismatch: %d != %d", r1.MaxDoc(), r2.MaxDoc())
	}
	if r1.DocCount() != r2.DocCount() {
		t.Errorf("DocCount mismatch: %d != %d", r1.DocCount(), r2.DocCount())
	}

	// Compare segments
	infos1 := r1.GetSegmentInfos()
	infos2 := r2.GetSegmentInfos()

	if infos1.Size() != infos2.Size() {
		t.Errorf("Segment count mismatch: %d != %d", infos1.Size(), infos2.Size())
	}

	for i := 0; i < infos1.Size(); i++ {
		sci1 := infos1.Get(i)
		sci2 := infos2.Get(i)

		si1 := sci1.SegmentInfo()
		si2 := sci2.SegmentInfo()

		if si1.Name() != si2.Name() {
			t.Errorf("Segment %d name mismatch: %s != %s", i, si1.Name(), si2.Name())
		}
		if si1.DocCount() != si2.DocCount() {
			t.Errorf("Segment %d DocCount mismatch: %d != %d", i, si1.DocCount(), si2.DocCount())
		}
		if sci1.DelCount() != sci2.DelCount() {
			t.Errorf("Segment %d DelCount mismatch: %d != %d", i, sci1.DelCount(), sci2.DelCount())
		}
	}

	// Compare FieldInfos across segments
	readers1 := r1.GetSegmentReaders()
	readers2 := r2.GetSegmentReaders()

	if len(readers1) != len(readers2) {
		t.Errorf("Segment reader count mismatch: %d != %d", len(readers1), len(readers2))
		return
	}

	for i := 0; i < len(readers1); i++ {
		fi1 := readers1[i].GetFieldInfos()
		fi2 := readers2[i].GetFieldInfos()

		if fi1 == nil && fi2 == nil {
			continue
		}
		if fi1 == nil || fi2 == nil {
			t.Errorf("FieldInfos mismatch in segment %d: one is nil", i)
			continue
		}

		if fi1.Size() != fi2.Size() {
			t.Errorf("FieldInfos size mismatch in segment %d: %d != %d", i, fi1.Size(), fi2.Size())
		}

		names1 := fi1.Names()
		names2 := fi2.Names()

		if len(names1) != len(names2) {
			t.Errorf("Field names count mismatch in segment %d: %d != %d", i, len(names1), len(names2))
			continue
		}

		for j := 0; j < len(names1); j++ {
			if names1[j] != names2[j] {
				t.Errorf("Field name mismatch in segment %d at index %d: %s != %s", i, j, names1[j], names2[j])
				continue
			}

			info1 := fi1.GetByName(names1[j])
			info2 := fi2.GetByName(names2[j])

			if info1.Number() != info2.Number() {
				t.Errorf("Field %s number mismatch in segment %d: %d != %d", names1[j], i, info1.Number(), info2.Number())
			}
			if info1.IsStored() != info2.IsStored() {
				t.Errorf("Field %s stored mismatch in segment %d: %v != %v", names1[j], i, info1.IsStored(), info2.IsStored())
			}
			if info1.IsTokenized() != info2.IsTokenized() {
				t.Errorf("Field %s tokenized mismatch in segment %d: %v != %v", names1[j], i, info1.IsTokenized(), info2.IsTokenized())
			}
			if info1.IndexOptions() != info2.IndexOptions() {
				t.Errorf("Field %s IndexOptions mismatch in segment %d: %v != %v", names1[j], i, info1.IndexOptions(), info2.IndexOptions())
			}
		}
	}
}

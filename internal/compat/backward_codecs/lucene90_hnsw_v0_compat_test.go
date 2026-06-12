// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene90_hnsw_v0_compat_test.go is the cross-engine compatibility anchor
// for the Lucene90 HNSW vectors v0 format write path. Gocene writes the
// segment using Lucene90HnswVectorsFormat while keeping every other format at
// the Lucene104 level; Lucene 10.4.0's CheckIndex is then run over the
// directory to prove the HNSW v0 vectors can be read back.
package backward_codecs

import (
	"errors"
	"fmt"
	"os/exec"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/backward_codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	gcompat "github.com/FlavioCFOliveira/Gocene/internal/compat"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// lucene90HnswVectorsFormat is a minimal KnnVectorsFormat that returns the
// Lucene90HnswVectorsWriter on the write path. The read path is intentionally
// unsupported because this is a test-only fixture writer; Lucene 10.4.0's own
// backward-codecs reader is used for validation.
type lucene90HnswVectorsFormat struct{}

func (f *lucene90HnswVectorsFormat) Name() string {
	return "Lucene90HnswVectorsFormat"
}

func (f *lucene90HnswVectorsFormat) FieldsWriter(state *codecs.SegmentWriteState) (codecs.KnnVectorsWriter, error) {
	return lucene90.NewLucene90HnswVectorsWriter(state, 16, 100)
}

func (f *lucene90HnswVectorsFormat) FieldsReader(state *codecs.SegmentReadState) (codecs.KnnVectorsReader, error) {
	return nil, errors.New("lucene90 hnsw: read not supported in test writer")
}

// lucene90HnswCodec delegates every format to Lucene104Codec except KNN
// vectors, which are handled by Lucene90HnswVectorsFormat.
// PerFieldKnnVectorsFormat is used so the concrete format name is recorded on
// each FieldInfo and can be resolved by Lucene on the read path.
type lucene90HnswCodec struct {
	*codecs.Lucene104Codec
}

func (c *lucene90HnswCodec) KnnVectorsFormat() codecs.KnnVectorsFormat {
	return codecs.NewPerFieldKnnVectorsFormatWithDefault(&lucene90HnswVectorsFormat{})
}

// TestLucene90HnswV0_GoceneWriteJavaCheck indexes a small corpus with
// Gocene's Lucene90HnswVectorsWriter and asks the Java harness to run
// CheckIndex. A clean exit proves Lucene 10.4.0 can read the HNSW v0 vector
// data, metadata, and graph produced by Gocene.
func TestLucene90HnswV0_GoceneWriteJavaCheck(t *testing.T) {
	requireHarness(t)

	dir := t.TempDir()
	d, err := store.NewSimpleFSDirectory(dir)
	if err != nil {
		t.Fatalf("open dir: %v", err)
	}
	defer d.Close()

	config := index.NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	config.SetCodec(&lucene90HnswCodec{Lucene104Codec: codecs.NewLucene104Codec()})

	iw, err := index.NewIndexWriter(d, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", fmt.Sprintf("doc-%d", i), true)
		doc.Add(idField)
		vecField, _ := document.NewKnnFloatVectorFieldEuclidean("vec", []float32{float32(i), float32(i + 1), float32(i + 2)})
		doc.Add(vecField)
		if err := iw.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}

	if err := iw.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := iw.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	jar, err := gcompat.Locate()
	if err != nil {
		t.Fatalf("locate harness: %v", err)
	}
	cmd := exec.Command("java", "-jar", jar, "check", dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("harness check %s failed: %v\noutput: %s", dir, err, out)
	}
}

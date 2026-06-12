// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene99_scalar_quantized_compat_test.go is the cross-engine compatibility
// anchor for the Lucene99 scalar-quantized vectors format write path. Gocene
// writes the segment using a custom codec that delegates KNN vectors to
// Lucene99ScalarQuantizedVectorsWriter while keeping every other format at the
// Lucene104 level; Lucene 10.4.0's CheckIndex is then run over the directory
// to prove the quantized vectors can be read back.
package backward_codecs

import (
	"errors"
	"fmt"
	"os/exec"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	gcompat "github.com/FlavioCFOliveira/Gocene/internal/compat"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// lucene99ScalarQuantizedVectorsFormat is a minimal KnnVectorsFormat that
// returns the Lucene99ScalarQuantizedVectorsWriter on the write path. The
// read path is intentionally unsupported because this is a test-only fixture
// writer; Lucene 10.4.0's own backward-codecs reader is used for validation.
type lucene99ScalarQuantizedVectorsFormat struct{}

func (f *lucene99ScalarQuantizedVectorsFormat) Name() string {
	return "Lucene99ScalarQuantizedVectorsFormat"
}

func (f *lucene99ScalarQuantizedVectorsFormat) FieldsWriter(state *codecs.SegmentWriteState) (codecs.KnnVectorsWriter, error) {
	return codecs.NewLucene99ScalarQuantizedVectorsWriter(state, nil)
}

func (f *lucene99ScalarQuantizedVectorsFormat) FieldsReader(state *codecs.SegmentReadState) (codecs.KnnVectorsReader, error) {
	return nil, errors.New("lucene99 scalar quantized: read not supported in test writer")
}

// lucene99ScalarQuantizedCodec delegates every format to Lucene104Codec except
// KNN vectors, which are handled by Lucene99ScalarQuantizedVectorsFormat.
// PerFieldKnnVectorsFormat is used so the concrete format name is recorded on
// each FieldInfo and can be resolved by Lucene on the read path.
type lucene99ScalarQuantizedCodec struct {
	*codecs.Lucene104Codec
}

func (c *lucene99ScalarQuantizedCodec) KnnVectorsFormat() codecs.KnnVectorsFormat {
	return codecs.NewPerFieldKnnVectorsFormatWithDefault(&lucene99ScalarQuantizedVectorsFormat{})
}

// TestLucene99ScalarQuantized_GoceneWriteJavaCheck indexes a small corpus with
// Gocene's Lucene99ScalarQuantizedVectorsWriter and asks the Java harness to
// run CheckIndex. A clean exit proves Lucene 10.4.0 can read the quantized
// vector data, metadata, and raw delegate files produced by Gocene.
func TestLucene99ScalarQuantized_GoceneWriteJavaCheck(t *testing.T) {
	requireHarness(t)

	dir := t.TempDir()
	d, err := store.NewSimpleFSDirectory(dir)
	if err != nil {
		t.Fatalf("open dir: %v", err)
	}
	defer d.Close()

	config := index.NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	config.SetCodec(&lucene99ScalarQuantizedCodec{Lucene104Codec: codecs.NewLucene104Codec()})

	iw, err := index.NewIndexWriter(d, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", fmt.Sprintf("doc-%d", i), true)
		doc.Add(idField)
		vecField, _ := document.NewKnnFloatVectorFieldEuclidean("vec", []float32{float32(i), float32(i+1), float32(i+2)})
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
	out, err := runCheckIndex(t, jar, dir)
	if err != nil {
		t.Fatalf("harness check %s failed: %v\noutput: %s", dir, err, out)
	}
}

func runCheckIndex(t *testing.T, jar, dir string) (string, error) {
	t.Helper()
	cmd := exec.Command("java", "-jar", jar, "check", dir)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
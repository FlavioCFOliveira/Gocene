// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene70_si_compat_test.go is the cross-engine compatibility anchor
// for the Lucene70 segment info format write path. Gocene writes the .si
// file using Lucene70SegmentInfoFormat while keeping every other format
// at the Lucene104 level; Lucene 10.4.0's CheckIndex is then run over
// the directory to prove the segment info can be read back.
package backward_codecs

import (
	"fmt"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/backward_codecs/lucene70"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// lucene70SegmentInfoCodec delegates every format to Lucene104Codec except
// segment info, which is handled by Lucene70SegmentInfoFormat. The codec
// name remains "Lucene104" so Lucene 10.4.0 opens the segment with its own
// Lucene104Codec; the .si file is then validated by CheckIndex.
type lucene70SegmentInfoCodec struct {
	*codecs.Lucene104Codec
}

// SegmentInfoFormat returns Lucene70SegmentInfoFormat.
func (c *lucene70SegmentInfoCodec) SegmentInfoFormat() codecs.SegmentInfoFormat {
	return lucene70.NewLucene70SegmentInfoFormat()
}

// TestLucene70SegmentInfo_GoceneWriteRejection verifies that Gocene's
// Lucene70SegmentInfoFormat rejects the write path, matching Apache Lucene
// 10.4.0 where old formats are read-only. Lucene 10.4.0's
// Lucene70SegmentInfoFormat.write throws UnsupportedOperationException;
// Gocene mirrors that by returning an error from Commit.
func TestLucene70SegmentInfo_GoceneWriteRejection(t *testing.T) {
	requireHarness(t)

	dir := t.TempDir()
	d, err := store.NewSimpleFSDirectory(dir)
	if err != nil {
		t.Fatalf("open dir: %v", err)
	}
	defer d.Close()

	analyzer := analysis.NewStandardAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	config.SetCodec(&lucene70SegmentInfoCodec{Lucene104Codec: codecs.NewLucene104Codec()})

	iw, err := index.NewIndexWriter(d, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer iw.Close()

	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", fmt.Sprintf("doc-%d", i), true)
		doc.Add(idField)
		bodyField, _ := document.NewTextField("body",
			fmt.Sprintf("alpha beta gamma delta %d epsilon zeta", i), true)
		doc.Add(bodyField)
		if err := iw.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}

	if err := iw.Commit(); err == nil {
		t.Fatalf("expected Commit to fail because Lucene70 segment-info format is read-only, got nil")
	} else if !strings.Contains(err.Error(), "old formats") && !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only error, got: %v", err)
	}
}

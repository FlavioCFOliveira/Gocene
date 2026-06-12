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
	"os/exec"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	gcompat "github.com/FlavioCFOliveira/Gocene/internal/compat"
	"github.com/FlavioCFOliveira/Gocene/backward_codecs/lucene70"
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

// TestLucene70SegmentInfo_GoceneWriteJavaCheck indexes a small corpus with
// Gocene's Lucene70SegmentInfoFormat and asks the Java harness to run
// CheckIndex. A clean exit proves Lucene 10.4.0 can read the .si file
// produced by Gocene.
func TestLucene70SegmentInfo_GoceneWriteJavaCheck(t *testing.T) {
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

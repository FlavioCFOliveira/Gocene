// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene103_postings_compat_test.go is the cross-engine compatibility anchor
// for the Lucene103 postings format write path. Gocene writes the segment
// using Lucene103RWPostingsFormat while keeping every other format at the
// Lucene104 level; Lucene 10.4.0's CheckIndex is then run over the
// directory to prove the postings can be read back.
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
	"github.com/FlavioCFOliveira/Gocene/store"
)

// lucene103PostingsCodec delegates every format to Lucene104Codec except
// postings, which are handled by Lucene103RWPostingsFormat. The codec name
// remains "Lucene104" so Lucene 10.4.0 opens the segment with its own
// Lucene104Codec; PerFieldPostingsFormat then dispatches to
// Lucene103RWPostingsFormat for the fields that were written with it.
type lucene103PostingsCodec struct {
	*codecs.Lucene104Codec
}

// PostingsFormat returns PerFieldPostingsFormat with Lucene103RWPostingsFormat
// as the default delegate. Using PerFieldPostingsFormat is required so the
// concrete postings format name is recorded on each FieldInfo and can be
// resolved by Lucene on the read path.
func (c *lucene103PostingsCodec) PostingsFormat() codecs.PostingsFormat {
	return codecs.NewPerFieldPostingsFormatWithDefault(codecs.NewLucene103RWPostingsFormat())
}

// TestLucene103Postings_GoceneWriteJavaCheck indexes a small corpus with
// Gocene's Lucene103RWPostingsFormat and asks the Java harness to run
// CheckIndex. A clean exit proves Lucene 10.4.0 can read the postings
// stream, term dictionary, and segment envelope produced by Gocene.
func TestLucene103Postings_GoceneWriteJavaCheck(t *testing.T) {
	requireHarness(t)

	dir := t.TempDir()
	d, err := store.NewSimpleFSDirectory(dir)
	if err != nil {
		t.Fatalf("open dir: %v", err)
	}
	defer d.Close()

	analyzer := analysis.NewStandardAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	config.SetCodec(&lucene103PostingsCodec{Lucene104Codec: codecs.NewLucene104Codec()})

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

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
)

// compatCodec wraps Lucene104Codec with Lucene90StoredFieldsFormat so that
// Gocene-written indexes are readable by Apache Lucene 10.4.0.
type checkIndexCompatCodec struct {
	codecs.Codec
	sf codecs.StoredFieldsFormat
}

func newCheckIndexCompatCodec() *checkIndexCompatCodec {
	return &checkIndexCompatCodec{
		Codec: codecs.NewLucene104Codec(),
		sf:    lucene90.NewLucene90StoredFieldsFormat(),
	}
}

func (c *checkIndexCompatCodec) StoredFieldsFormat() codecs.StoredFieldsFormat {
	return c.sf
}

// TestCheckIndex_GoceneWrite generates a small index with Gocene and asserts
// that Lucene 10.4.0 CheckIndex reports it clean.  This is the inverse
// direction of the existing fixture-based CheckIndex tests (Lucene-write →
// Gocene-read).
func TestCheckIndex_GoceneWrite(t *testing.T) {
	for _, seed := range []int64{0xC0FFEE, 0xDECAF} {
		seed := seed
		t.Run(strconv.FormatInt(seed, 10), func(t *testing.T) {
			requireHarness(t)

			dir := t.TempDir()
			fsDir, err := store.NewSimpleFSDirectory(dir)
			if err != nil {
				t.Fatalf("open dir: %v", err)
			}
			defer fsDir.Close()

			// Temporarily register the compat codec as the default so that
			// IndexWriter and any nested writers (e.g., taxonomy) use it.
			compat := newCheckIndexCompatCodec()
			origCodec := index.GetDefaultCodec()
			index.RegisterNamedCodec("Lucene104", compat)
			index.RegisterDefaultCodec(compat)
			defer func() {
				index.RegisterNamedCodec("Lucene104", origCodec)
				index.RegisterDefaultCodec(origCodec)
			}()

			cfg := index.NewIndexWriterConfig(analysis.NewStandardAnalyzer())
			cfg.SetUseCompoundFile(false)
			cfg.SetMergePolicy(index.NewNoMergePolicy())
			cfg.SetMergeScheduler(index.NewSerialMergeScheduler())
			cfg.SetCodec(compat)

			iw, err := index.NewIndexWriter(fsDir, cfg)
			if err != nil {
				t.Fatalf("NewIndexWriter: %v", err)
			}

			// Index a deterministic corpus.
			words := []string{"alpha", "beta", "gamma", "delta"}
			for i := 0; i < 8; i++ {
				doc := document.NewDocument()
				idField, _ := document.NewStoredField("id", strconv.Itoa(i))
				doc.Add(idField)
				body := words[i%len(words)] + " " + words[(i+1)%len(words)]
				bodyField, _ := document.NewTextField("body", body, false)
				doc.Add(bodyField)
				if err := iw.AddDocument(doc); err != nil {
					t.Fatalf("AddDocument: %v", err)
				}
			}

			if err := iw.Commit(); err != nil {
				t.Fatalf("Commit: %v", err)
			}
			iw.Close()

			out, err := checkIndex(t, dir)
			if err != nil {
				t.Fatalf("CheckIndex non-clean on Gocene-written index: %v\n%s", err, out)
			}
		})
	}
}

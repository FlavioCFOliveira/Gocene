// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package scenarios

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	gcompat "github.com/FlavioCFOliveira/Gocene/internal/compat"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

const s2TsvName = "s2-hits.tsv"

// TestS2_GoceneWriteLeg generates the combined S2 index from Gocene (single
// segment) and writes s2-hits.tsv. The Java harness verifies that the
// Lucene-side re-scoring matches and that S2 hits are byte-identical to S1.
func TestS2_GoceneWriteLeg(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run(strconv.FormatInt(seed, 10), func(t *testing.T) {
			dir := t.TempDir()

			fsDir, err := store.NewSimpleFSDirectory(dir)
			if err != nil {
				t.Fatalf("open dir: %v", err)
			}
			defer fsDir.Close()

			cfg := index.NewIndexWriterConfig(analysis.NewStandardAnalyzer())
			cfg.SetUseCompoundFile(false)
			cfg.SetMergePolicy(index.NewNoMergePolicy())
			cfg.SetMergeScheduler(index.NewSerialMergeScheduler())
			cfg.SetCodec(newCompatCodec())

			iw, err := index.NewIndexWriter(fsDir, cfg)
			if err != nil {
				t.Fatalf("NewIndexWriter: %v", err)
			}

			for i := 0; i < s1NumDocs; i++ {
				doc, err := s1BuildDoc(i, seed)
				if err != nil {
					t.Fatalf("buildDoc(%d): %v", i, err)
				}
				if err := iw.AddDocument(doc); err != nil {
					t.Fatalf("AddDocument: %v", err)
				}
			}
			if err := iw.Commit(); err != nil {
				t.Fatalf("Commit: %v", err)
			}
			iw.Close()

			reader, err := index.OpenDirectoryReader(fsDir)
			if err != nil {
				t.Fatalf("OpenDirectoryReader: %v", err)
			}
			defer reader.Close()

			leaves, err := reader.Leaves()
			if err != nil {
				t.Fatalf("Leaves: %v", err)
			}
			if len(leaves) != 1 {
				t.Fatalf("expected 1 segment, got %d", len(leaves))
			}

			rows, err := s1Evaluate(reader)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}

			if err := s1WriteTsv(filepath.Join(dir, s2TsvName), rows); err != nil {
				t.Fatalf("writeTsv: %v", err)
			}

			if err := gcompat.Verify(scenarioS2, seed, dir); err != nil {
				t.Fatalf("harness verify: %v", err)
			}
		})
	}
}

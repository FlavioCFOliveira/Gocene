// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

package index

import (
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestDVUpdates_GoceneWrite generates a small index, performs a doc-values update,
// and asserts that the resulting index is readable by Lucene (via CheckIndex).
//
// This is the write-path leg of the DV-update round-trip: Gocene-write -> Lucene-read.
func TestDVUpdates_GoceneWrite(t *testing.T) {
	for _, seed := range []int64{0xDEADBEEF, 0xCAFEBABE} {
		seed := seed
		t.Run(strconv.FormatInt(seed, 10), func(t *testing.T) {
			requireHarness(t)

			dir := t.TempDir()
			fsDir, err := store.NewSimpleFSDirectory(dir)
			if err != nil {
				t.Fatalf("open dir: %v", err)
			}
			defer fsDir.Close()

			// Use Lucene104 codec for binary compatibility.
			compat := codecs.NewLucene104Codec()
			origCodec := index.GetDefaultCodec()
			index.RegisterDefaultCodec(compat)
			defer func() {
				index.RegisterDefaultCodec(origCodec)
			}()

			cfg := index.NewIndexWriterConfigWithAnalyzer(analysis.NewStandardAnalyzer())
			cfg.SetCodec(compat)
			iw, err := index.NewIndexWriter(fsDir, cfg)
			if err != nil {
				t.Fatalf("NewIndexWriter: %v", err)
			}

			// 1. Index documents with a numeric doc-values field "count".
			for i := 0; i < 10; i++ {
				doc := document.NewDocument()
				// Field "count" as a numeric doc-values field.
				countField, _ := document.NewIntField("count", i, true)
				doc.Add(countField)
				if _, err := iw.AddDocument(doc); err != nil {
					t.Fatalf("AddDocument: %v", err)
				}
			}

			if _, err := iw.Commit(); err != nil {
				t.Fatalf("Commit: %v", err)
			}
			iw.Close()

			// 2. Perform a DocValues update using ReadersAndUpdates.
			reader, err := index.OpenDirectoryReader(fsDir)
			if err != nil {
				t.Fatalf("OpenDirectoryReader: %v", err)
			}
			segReader := reader.GetSegmentReaders()[0]
			info := segReader.GetSegmentCommitInfo()
			reader.Close()

			// PendingDeletes(SegmentCommitInfo) delegates with liveDocs == null
			// and liveDocsInitialized == !info.hasDeletions().
			rau, err := index.NewReadersAndUpdates(10, info, index.NewPendingDeletes(info, nil, !info.HasDeletions()))
			if err != nil {
				t.Fatalf("NewReadersAndUpdates: %v", err)
			}

			update, err := index.NewNumericDocValuesFieldUpdates(
				1, // delGen 1
				"count",
				info.Info.MaxDoc(),
			)
			if err != nil {
				t.Fatalf("NewNumericDocValuesFieldUpdates: %v", err)
			}
			if err := update.AddLong(5, 999); err != nil {
				t.Fatalf("AddLong: %v", err)
			}
			if err := update.Finish(); err != nil {
				t.Fatalf("Finish: %v", err)
			}

			if err := rau.AddDVUpdate(update); err != nil {
				t.Fatalf("AddDVUpdate: %v", err)
			}

			// 3. Trigger the write path.
			fn := index.NewFieldNumbers("", "")
			fi := index.NewFieldInfo("count", -1, index.FieldInfoOptions{
				DocValuesType: index.DocValuesTypeNumeric,
			})
			fn.AddOrGet(fi)

			written, err := rau.WriteFieldUpdates(fsDir, fn, 1, nil)
			if err != nil {
				t.Fatalf("WriteFieldUpdates: %v", err)
			}
			if !written {
				t.Fatal("WriteFieldUpdates reported no changes, but updates were added")
			}

			// 4. Verify files are created.
			files := listFiles(t, dir, false)
			var sawDvd, sawDvm bool
			for _, n := range files {
				if strings.Contains(n, "_1_Lucene104") && strings.HasSuffix(n, ".dvd") {
					sawDvd = true
				}
				if strings.Contains(n, "_1_Lucene104") && strings.HasSuffix(n, ".dvm") {
					sawDvm = true
				}
			}
			if !sawDvd || !sawDvm {
				t.Errorf("missing generational DV files after WriteFieldUpdates; got %v", files)
			}

			// 5. Verify that Lucene's CheckIndex considers the index clean.
			out, err := checkIndex(t, dir)
			if err != nil {
				t.Fatalf("CheckIndex non-clean on Gocene-written DV update: %v\n%s", err, out)
			}
		})
	}
}

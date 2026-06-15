// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package benchmark

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func BenchmarkSearchThroughput(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	cfg.SetUseCompoundFile(false)
	iw, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		b.Fatal(err)
	}
	defer iw.Close()

	const numDocs = 10000
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		field, err := document.NewTextField("content", randomText(128), true)
		if err != nil {
			b.Fatal(err)
		}
		doc.Add(field)
		if err := iw.AddDocument(doc); err != nil {
			b.Fatal(err)
		}
	}
	if err := iw.Commit(); err != nil {
		b.Fatal(err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		b.Fatal(err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	defer searcher.Close()

	query := search.NewTermQuery(index.NewTerm("content", "a"))

	for _, topN := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("topN=%d", topN), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				topDocs, err := searcher.Search(query, topN)
				if err != nil {
					b.Fatal(err)
				}
				_ = topDocs
			}
			b.StopTimer()
			queriesPerSec := float64(b.N) / b.Elapsed().Seconds()
			b.ReportMetric(queriesPerSec, "queries/sec")
		})
	}
}

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package benchmark

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func BenchmarkIndexThroughput(b *testing.B) {
	for _, docsPerBatch := range []int{1, 10, 100, 1000} {
		b.Run(fmt.Sprintf("docsPerBatch=%d", docsPerBatch), func(b *testing.B) {
			dir := store.NewByteBuffersDirectory()
			defer dir.Close()

			cfg := index.NewIndexWriterConfig(analysis.NewStandardAnalyzer())
			cfg.SetUseCompoundFile(false)
			iw, err := index.NewIndexWriter(dir, cfg)
			if err != nil {
				b.Fatal(err)
			}
			defer iw.Close()

			totalDocs := b.N * docsPerBatch
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				batch := make([]index.Document, docsPerBatch)
				for j := 0; j < docsPerBatch; j++ {
					doc := document.NewDocument()
					field, err := document.NewTextField("content", randomText(256), true)
					if err != nil {
						b.Fatal(err)
					}
					doc.Add(field)
					batch[j] = doc
				}
				if err := iw.AddDocuments(batch); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()

			docsPerSec := float64(totalDocs) / b.Elapsed().Seconds()
			b.ReportMetric(docsPerSec, "docs/sec")
		})
	}
}

func randomText(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

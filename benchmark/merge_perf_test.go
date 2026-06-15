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
	"github.com/FlavioCFOliveira/Gocene/store"
)

func BenchmarkMergePerformance(b *testing.B) {
	for _, numSegments := range []int{2, 5, 10} {
		b.Run(fmt.Sprintf("segments=%d", numSegments), func(b *testing.B) {
			dir := store.NewByteBuffersDirectory()
			defer dir.Close()

			cfg := index.NewIndexWriterConfig(analysis.NewStandardAnalyzer())
			cfg.SetUseCompoundFile(false)
			cfg.SetMergeScheduler(index.NewSerialMergeScheduler())

			for s := 0; s < numSegments; s++ {
				iw, err := index.NewIndexWriter(dir, cfg)
				if err != nil {
					b.Fatal(err)
				}
				for i := 0; i < 1000; i++ {
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
				iw.Close()
			}

			iw, err := index.NewIndexWriter(dir, cfg)
			if err != nil {
				b.Fatal(err)
			}
			defer iw.Close()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := iw.ForceMerge(1); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			mergesPerSec := float64(b.N) / b.Elapsed().Seconds()
			b.ReportMetric(mergesPerSec, "merges/sec")
		})
	}
}

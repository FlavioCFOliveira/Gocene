// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// BenchmarkFieldInfosSerialisation measures the throughput of constructing and
// querying a FieldInfos collection representative of a typical Lucene segment
// (32 fields). The benchmark covers Add (with lock), Freeze, GetByName, and
// GetByNumber — the operations performed during segment flush and merge.
func BenchmarkFieldInfosSerialisation(b *testing.B) {
	const numFields = 32

	opts := index.DefaultFieldInfoOptions()
	opts.IndexOptions = index.IndexOptionsDocs
	opts.Stored = true
	opts.Tokenized = true

	// Pre-build field names to avoid fmt overhead inside the loop.
	names := make([]string, numFields)
	for i := range names {
		names[i] = fmt.Sprintf("field_%02d", i)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		fi := index.NewFieldInfos()
		for j := 0; j < numFields; j++ {
			info := index.NewFieldInfo(names[j], j, opts)
			if err := fi.Add(info); err != nil {
				b.Fatal(err)
			}
		}
		fi.Freeze()

		// Simulate read path (segment reader field lookup).
		for j := 0; j < numFields; j++ {
			if fi.GetByName(names[j]) == nil {
				b.Fatalf("field %s not found", names[j])
			}
			if fi.GetByNumber(j) == nil {
				b.Fatalf("field number %d not found", j)
			}
		}
	}
}

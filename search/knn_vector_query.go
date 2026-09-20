// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// This file has no counterpart in the Apache Lucene 10.5.0 source tree. The
// KnnFloatVectorQuery and KnnByteVectorQuery ports live in the files named for
// their Lucene classes, knn_float_vector_query.go and knn_byte_vector_query.go;
// what remains here are the Go-only helpers those ports were built on.

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// knnFloatLeafSearcher is the structural per-leaf search surface the float
// KNN query drives. *index.SegmentReader satisfies it (via the codec KNN
// reader wiring landed by rmp #4731).
type knnFloatLeafSearcher interface {
	SearchNearestVectors(field string, target []float32, k int, acceptDocs util.Bits) (index.TopDocs, error)
}

// knnByteLeafSearcher is the byte analogue of [knnFloatLeafSearcher].
type knnByteLeafSearcher interface {
	SearchNearestVectorsByte(field string, target []byte, k int, acceptDocs util.Bits) (index.TopDocs, error)
}

// indexTopDocsToSearch converts the index-package TopDocs returned by a leaf
// reader's vector search into a search-package *TopDocs. Per-leaf vector
// results are exact (EQUAL_TO) for the result count reported.
func indexTopDocsToSearch(td index.TopDocs) *TopDocs {
	scoreDocs := make([]*ScoreDoc, len(td.ScoreDocs))
	for i, sd := range td.ScoreDocs {
		scoreDocs[i] = &ScoreDoc{Doc: sd.Doc, Score: sd.Score}
	}
	return NewTopDocs(NewTotalHits(int64(len(scoreDocs)), EQUAL_TO), scoreDocs)
}

// Compile-time guards that the queries satisfy the KnnVectorQueryImpl
// contract (and therefore Query).
var (
	_ KnnVectorQueryImpl = (*KnnFloatVectorQuery)(nil)
	_ KnnVectorQueryImpl = (*KnnByteVectorQuery)(nil)
)

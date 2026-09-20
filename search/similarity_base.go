// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// SimilarityBase ports
// lucene/core/src/java/org/apache/lucene/search/similarities/SimilarityBase.java.
// The declaration that used to sit here carried none of that class's surface
// (no score, explain, scorer, computeNorm or fillBasicStats); the port with
// those members is in similarity_base_lucene.go, which is where SimilarityBase
// and NewSimilarityBase are declared.

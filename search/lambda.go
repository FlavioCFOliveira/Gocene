// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "math"

// LuceneIBLambda mirrors org.apache.lucene.search.similarities.Lambda —
// the lambda_w parameter for the IB framework.
type LuceneIBLambda interface {
	// Lambda returns the lambda_w value.
	Lambda(stats *LuceneBasicStats) float32

	// Explain returns the canonical Explanation tree.
	Explain(stats *LuceneBasicStats) Explanation

	// String returns the lambda code ("D", "L").
	String() string
}

// LuceneLambdaDF mirrors LambdaDF — (docFreq+1) / (numberOfDocuments+1).
type LuceneLambdaDF struct{}

// NewLuceneLambdaDF constructs the parameter-free LambdaDF.
func NewLuceneLambdaDF() *LuceneLambdaDF { return &LuceneLambdaDF{} }

// Lambda implements LuceneIBLambda. Mirrors Java's nextDown(1.0) trick to
// dodge the SPL singularity at lambda == 1.
func (LuceneLambdaDF) Lambda(stats *LuceneBasicStats) float32 {
	lambda := float32(float64(stats.DocFreq()+1) / float64(stats.NumberOfDocuments()+1))
	if lambda == 1 {
		lambda = math.Nextafter32(lambda, float32(math.Inf(-1)))
	}
	return lambda
}

// Explain returns the canonical LambdaDF tree.
func (l LuceneLambdaDF) Explain(stats *LuceneBasicStats) Explanation {
	exp := NewExplanation(true, l.Lambda(stats),
		"LambdaDF, computed as (n + 1) / (N + 1) from:")
	exp.AddDetail(NewExplanation(true, float32(stats.DocFreq()), "n, number of documents containing term"))
	exp.AddDetail(NewExplanation(true, float32(stats.NumberOfDocuments()), "N, total number of documents with field"))
	return exp
}

// String returns "D".
func (LuceneLambdaDF) String() string { return "D" }

// LuceneLambdaTTF mirrors LambdaTTF — (totalTermFreq+1) / (numberOfDocuments+1).
type LuceneLambdaTTF struct{}

// NewLuceneLambdaTTF constructs the parameter-free LambdaTTF.
func NewLuceneLambdaTTF() *LuceneLambdaTTF { return &LuceneLambdaTTF{} }

// Lambda implements LuceneIBLambda. Mirrors Java's nextUp(1.0) trick.
func (LuceneLambdaTTF) Lambda(stats *LuceneBasicStats) float32 {
	lambda := float32(float64(stats.TotalTermFreq()+1) / float64(stats.NumberOfDocuments()+1))
	if lambda == 1 {
		lambda = math.Nextafter32(lambda, float32(math.Inf(+1)))
	}
	return lambda
}

// Explain returns the canonical LambdaTTF tree.
func (l LuceneLambdaTTF) Explain(stats *LuceneBasicStats) Explanation {
	exp := NewExplanation(true, l.Lambda(stats),
		"LambdaTTF, computed as (F + 1) / (N + 1) from:")
	exp.AddDetail(NewExplanation(true, float32(stats.TotalTermFreq()), "F, total number of occurrences of term across all documents"))
	exp.AddDetail(NewExplanation(true, float32(stats.NumberOfDocuments()), "N, total number of documents with field"))
	return exp
}

// String returns "L".
func (LuceneLambdaTTF) String() string { return "L" }

// Compile-time guarantees.
var (
	_ LuceneIBLambda = LuceneLambdaDF{}
	_ LuceneIBLambda = LuceneLambdaTTF{}
)

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/payloads/SumPayloadFunction.java

package payloads

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// SumPayloadFunction calculates the final score as the sum of scores of all
// payloads seen.
//
// Mirrors org.apache.lucene.queries.payloads.SumPayloadFunction.
type SumPayloadFunction struct{}

// CurrentScore adds the current payload score to the cumulative score.
func (SumPayloadFunction) CurrentScore(_ int, _ string, _, _ int, _ int,
	currentScore, currentPayloadScore float32) float32 {
	return currentPayloadScore + currentScore
}

// DocScore returns the final payload score, or 1 if no payloads were seen.
func (SumPayloadFunction) DocScore(_ int, _ string, numPayloadsSeen int, payloadScore float32) float32 {
	if numPayloadsSeen > 0 {
		return payloadScore
	}
	return 1
}

// Explain returns an explanation of the payload score.
func (f SumPayloadFunction) Explain(_ int, _ string, numPayloadsSeen int, payloadScore float32) search.Explanation {
	return search.MatchExplanation(
		f.DocScore(0, "", numPayloadsSeen, payloadScore),
		"SumPayloadFunction.docScore()",
	)
}

// SumPayloadFunctionSingleton is a reusable instance.
var SumPayloadFunctionSingleton = &SumPayloadFunction{}

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/payloads/AveragePayloadFunction.java

package payloads

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// AveragePayloadFunction calculates the final score as the average score of
// all payloads seen.
//
// Mirrors org.apache.lucene.queries.payloads.AveragePayloadFunction.
type AveragePayloadFunction struct{}

// CurrentScore adds the current payload score to the cumulative score (sum).
func (AveragePayloadFunction) CurrentScore(_ int, _ string, _, _ int, _ int,
	currentScore, currentPayloadScore float32) float32 {
	return currentPayloadScore + currentScore
}

// DocScore returns the average of all payload scores, or 1 if no payloads were seen.
func (AveragePayloadFunction) DocScore(_ int, _ string, numPayloadsSeen int, payloadScore float32) float32 {
	if numPayloadsSeen > 0 {
		return payloadScore / float32(numPayloadsSeen)
	}
	return 1
}

// Explain returns an explanation of the payload score.
func (f AveragePayloadFunction) Explain(_ int, _ string, numPayloadsSeen int, payloadScore float32) search.Explanation {
	return search.MatchExplanation(
		f.DocScore(0, "", numPayloadsSeen, payloadScore),
		"AveragePayloadFunction.docScore()",
	)
}

// AveragePayloadFunctionSingleton is a reusable instance.
var AveragePayloadFunctionSingleton = &AveragePayloadFunction{}

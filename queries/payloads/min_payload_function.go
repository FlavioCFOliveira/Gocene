// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/payloads/MinPayloadFunction.java

package payloads

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// MinPayloadFunction calculates the minimum payload score seen.
//
// Mirrors org.apache.lucene.queries.payloads.MinPayloadFunction.
type MinPayloadFunction struct{}

// CurrentScore returns the minimum of the current payload and the cumulative score.
func (MinPayloadFunction) CurrentScore(_ int, _ string, _, _ int, numPayloadsSeen int,
	currentScore, currentPayloadScore float32) float32 {
	if numPayloadsSeen == 0 {
		return currentPayloadScore
	}
	return float32(math.Min(float64(currentPayloadScore), float64(currentScore)))
}

// DocScore returns the final payload score, or 1 if no payloads were seen.
func (MinPayloadFunction) DocScore(_ int, _ string, numPayloadsSeen int, payloadScore float32) float32 {
	if numPayloadsSeen > 0 {
		return payloadScore
	}
	return 1
}

// Explain returns an explanation of the payload score.
func (f MinPayloadFunction) Explain(_ int, _ string, numPayloadsSeen int, payloadScore float32) search.Explanation {
	return search.MatchExplanation(
		f.DocScore(0, "", numPayloadsSeen, payloadScore),
		"MinPayloadFunction.docScore()",
	)
}

// MinPayloadFunctionSingletons is a reusable instance.
var MinPayloadFunctionSingleton = &MinPayloadFunction{}

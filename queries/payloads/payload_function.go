// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/payloads/PayloadFunction.java

package payloads

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// PayloadFunction defines how payload scores are combined.
//
// Mirrors org.apache.lucene.queries.payloads.PayloadFunction (abstract class).
type PayloadFunction interface {
	// CurrentScore calculates the score up to this point for this doc and field.
	CurrentScore(docId int, field string, start, end, numPayloadsSeen int,
		currentScore, currentPayloadScore float32) float32

	// DocScore calculates the final score for all the payloads seen so far
	// for this doc/field.
	DocScore(docId int, field string, numPayloadsSeen int, payloadScore float32) float32

	// Explain returns an explanation of the payload score for the given document.
	Explain(docId int, field string, numPayloadsSeen int, payloadScore float32) search.Explanation
}

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/payloads/PayloadMatcher.java

package payloads

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// PayloadMatcher defines an interface for testing if two payloads should be
// considered to match.
//
// Mirrors org.apache.lucene.queries.payloads.PayloadMatcher.
type PayloadMatcher interface {
	// ComparePayload tests if two BytesRef match.
	// source is the left side of the compare, payload is the right side.
	ComparePayload(source, payload *util.BytesRef) bool
}

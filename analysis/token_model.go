// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// Token represents a single token in a stream.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.Token.
type Token struct {
	Term           string
	StartOffset    int
	EndOffset      int
	Position       int
	PositionInc    int
	PositionLength int
}

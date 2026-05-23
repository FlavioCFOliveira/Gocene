// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package email

// UAX29URLEmailTokenizerImpl is the scanner component for UAX29URLEmailTokenizer.
//
// This is the Go port of
// org.apache.lucene.analysis.email.UAX29URLEmailTokenizerImpl from
// Apache Lucene 10.4.0.
//
// Deviation: the Java reference is a JFlex-generated DFA (~33 000 lines).
// In Gocene, the scanning logic lives directly in UAX29URLEmailTokenizer
// (analysis package), which was ported using regexp-based scanning.
// This type is retained as a named alias so that callers that reference
// UAX29URLEmailTokenizerImpl by name continue to compile.
type UAX29URLEmailTokenizerImpl struct{}

// NewUAX29URLEmailTokenizerImpl creates a UAX29URLEmailTokenizerImpl.
//
// The real scanning state is managed internally by UAX29URLEmailTokenizer.
func NewUAX29URLEmailTokenizerImpl() *UAX29URLEmailTokenizerImpl {
	return &UAX29URLEmailTokenizerImpl{}
}

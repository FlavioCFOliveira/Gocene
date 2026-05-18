// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest

// InputIterator is the foundational iterator the suggest sub-packages
// consume. Mirrors org.apache.lucene.search.suggest.InputIterator.
//
// Each Next call advances to the next term and exposes its weight, optional
// payload, and contexts. ok=false signals end-of-stream.
type InputIterator interface {
	Next() (term []byte, weight int64, payload []byte, contexts [][]byte, ok bool, err error)

	// HasPayloads reports whether the iterator produces payloads.
	HasPayloads() bool

	// HasContexts reports whether the iterator produces contexts.
	HasContexts() bool
}

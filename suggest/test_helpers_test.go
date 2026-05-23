// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest_test

// This file contains test-support types that mirror the Java Lucene test
// helpers from suggest/src/test/.  They are not part of the production API.

// Input corresponds to org.apache.lucene.search.suggest.Input.
// Each instance captures the (term, weight, payload, contexts) quad that
// InputArrayIterator emits.
type Input struct {
	Term        []byte
	Weight      int64
	Payload     []byte
	Contexts    [][]byte
	HasPayload  bool
	HasContexts bool
}

// NewInput creates a bare (term, weight) Input — no payload, no contexts.
// Mirrors Input(BytesRef, long) and Input(String, long).
func NewInput(term string, weight int64) *Input {
	return &Input{Term: []byte(term), Weight: weight}
}

// NewInputWithPayload creates a (term, weight, payload) Input.
// Mirrors Input(BytesRef, long, BytesRef) and Input(String, long, BytesRef).
func NewInputWithPayload(term string, weight int64, payload []byte) *Input {
	p := make([]byte, len(payload))
	copy(p, payload)
	return &Input{Term: []byte(term), Weight: weight, Payload: p, HasPayload: true}
}

// NewInputWithContexts creates a (term, weight, contexts) Input.
// Mirrors Input(String, long, Set<BytesRef>).
func NewInputWithContexts(term string, weight int64, contexts ...string) *Input {
	ctxs := make([][]byte, len(contexts))
	for i, c := range contexts {
		ctxs[i] = []byte(c)
	}
	return &Input{Term: []byte(term), Weight: weight, Contexts: ctxs, HasContexts: len(ctxs) > 0}
}

// NewInputWithPayloadAndContexts creates a fully populated Input.
// Mirrors Input(String, int, BytesRef, Set<BytesRef>).
func NewInputWithPayloadAndContexts(term string, weight int64, payload []byte, contexts ...string) *Input {
	p := make([]byte, len(payload))
	copy(p, payload)
	ctxs := make([][]byte, len(contexts))
	for i, c := range contexts {
		ctxs[i] = []byte(c)
	}
	return &Input{
		Term:        []byte(term),
		Weight:      weight,
		Payload:     p,
		HasPayload:  true,
		Contexts:    ctxs,
		HasContexts: len(ctxs) > 0,
	}
}

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest_test

import "github.com/FlavioCFOliveira/Gocene/suggest"

// InputArrayIterator is an InputIterator over a slice of Input values.
// Mirrors org.apache.lucene.search.suggest.InputArrayIterator.
//
// hasPayloads and hasContexts are determined from the first element, matching
// the Java constructor's contract.
type InputArrayIterator struct {
	inputs      []*Input
	pos         int
	hasPayloads bool
	hasContexts bool
}

// NewInputArrayIterator creates an iterator over inputs.
// Mirrors InputArrayIterator(Input[]) and InputArrayIterator(Iterator<Input>).
func NewInputArrayIterator(inputs []*Input) *InputArrayIterator {
	it := &InputArrayIterator{inputs: inputs, pos: -1}
	if len(inputs) > 0 {
		it.hasPayloads = inputs[0].HasPayload
		it.hasContexts = inputs[0].HasContexts
	}
	return it
}

// Next advances the iterator and returns the next entry.
func (it *InputArrayIterator) Next() (term []byte, weight int64, payload []byte, contexts [][]byte, ok bool, err error) {
	it.pos++
	if it.pos >= len(it.inputs) {
		return nil, 0, nil, nil, false, nil
	}
	in := it.inputs[it.pos]
	t := make([]byte, len(in.Term))
	copy(t, in.Term)
	var p []byte
	if in.HasPayload {
		p = make([]byte, len(in.Payload))
		copy(p, in.Payload)
	}
	var ctxs [][]byte
	if in.HasContexts {
		ctxs = make([][]byte, len(in.Contexts))
		for i, c := range in.Contexts {
			ctxs[i] = make([]byte, len(c))
			copy(ctxs[i], c)
		}
	}
	return t, in.Weight, p, ctxs, true, nil
}

// HasPayloads reports whether this iterator carries payloads.
func (it *InputArrayIterator) HasPayloads() bool { return it.hasPayloads }

// HasContexts reports whether this iterator carries contexts.
func (it *InputArrayIterator) HasContexts() bool { return it.hasContexts }

var _ suggest.InputIterator = (*InputArrayIterator)(nil)

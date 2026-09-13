// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// CompiledAutomatonTermsEnum returns a TermsEnum intersecting the provided
// Terms with the terms accepted by the compiled automaton.
//
// Ported from org.apache.lucene.util.automaton.CompiledAutomaton#getTermsEnum
// (Apache Lucene 10.5.0).
//
// PORT NOTE. In Java this is a method on CompiledAutomaton, whose file sits in
// org.apache.lucene.util.automaton and imports org.apache.lucene.index.Terms,
// TermsEnum and SingleTermsEnum; Java tolerates the resulting package cycle and
// Go does not (spi, and therefore index, already import util/automaton). The
// member is therefore rendered where its parameter and result types live, as a
// package-level function — the same split search/sort_field.go applies to
// SortField#rewrite. The rest of the class stays in
// util/automaton/compiled_automaton.go.
//
// Java's SingleTermsEnum(TermsEnum, BytesRef) carries no field name because its
// only comparison is on term bytes. Gocene's TermsEnum is addressed by *Term
// (field plus bytes), so the field is taken from the Terms being intersected —
// which is by construction the field the returned enumeration runs over.
func CompiledAutomatonTermsEnum(compiled *automaton.CompiledAutomaton, terms Terms) (TermsEnum, error) {
	switch compiled.Type {
	case automaton.AutomatonTypeNone:
		return &EmptyTermsEnum{}, nil
	case automaton.AutomatonTypeAll:
		return terms.GetIterator()
	case automaton.AutomatonTypeSingle:
		in, err := terms.GetIterator()
		if err != nil {
			return nil, err
		}
		return NewSingleTermFilteredEnum(in, NewTermFromBytesRef(terms.Field(), compiled.Term)), nil
	case automaton.AutomatonTypeNormal:
		return terms.Intersect(compiled, nil)
	default:
		// unreachable
		return nil, fmt.Errorf("unhandled case: %v", compiled.Type)
	}
}

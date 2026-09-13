// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// CompiledAutomatonVisit reports back to a QueryVisitor how the compiled
// automaton matches terms.
//
// Ported from org.apache.lucene.util.automaton.CompiledAutomaton#visit
// (Apache Lucene 10.5.0).
//
// PORT NOTE. In Java this is a method on CompiledAutomaton, whose file sits in
// org.apache.lucene.util.automaton and imports org.apache.lucene.search.Query
// and QueryVisitor; Java tolerates the resulting package cycle and Go does not
// (search imports index, which imports spi, which imports util/automaton). The
// member is therefore rendered where its parameter types live, as a
// package-level function — the same split this package applies to
// SortField#rewrite in search/sort_field.go. The rest of the class stays in
// util/automaton/compiled_automaton.go.
func CompiledAutomatonVisit(compiled *automaton.CompiledAutomaton, visitor QueryVisitor, parent Query, field string) {
	if visitor.AcceptField(field) {
		switch compiled.Type {
		case automaton.AutomatonTypeNormal:
			visitor.ConsumeTermsMatching(parent, field, func() ByteRunAutomaton {
				return byteRunAutomatonAdapter{a: compiled.RunAutomaton}
			})
		case automaton.AutomatonTypeNone:
		case automaton.AutomatonTypeAll:
			visitor.ConsumeTermsMatching(parent, field, func() ByteRunAutomaton {
				return byteRunAutomatonAdapter{a: automaton.NewByteRunAutomaton(automaton.MakeAnyString())}
			})
		case automaton.AutomatonTypeSingle:
			visitor.ConsumeTerms(parent, index.NewTermFromBytesRef(field, compiled.Term))
		}
	}
}

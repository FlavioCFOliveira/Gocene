// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// AutomatonTermsEnum enumerates terms accepted by a CompiledAutomaton.
// Mirrors org.apache.lucene.index.AutomatonTermsEnum (Apache Lucene 10.4.0).
//
// Gocene skeleton: this initial port wires the FilteredTermsEnum-based
// scaffolding and the Accept callback (which delegates to the compiled
// automaton's ByteRunAutomaton). The intelligent NextSeekTerm optimization
// that skips past prefixes the automaton rejects en bloc is deferred to a
// follow-up task; see backlog #2704.
type AutomatonTermsEnum struct {
	*FilteredTermsEnum

	compiled *automaton.CompiledAutomaton
}

// NewAutomatonTermsEnum builds an AutomatonTermsEnum that filters delegate
// using the provided CompiledAutomaton.
//
// DIVERGENCE (documented, Source Fidelity Mandate point 5). Java's
// AutomatonTermsEnum calls `super(tenum)`, i.e. startWithSeek == true, because
// its nextSeekTerm is a real implementation that walks the DFA (nextString /
// setLinear / backtrack) and returns the first matching string. Gocene's
// NextSeekTerm below is still the stub recorded in this file's type comment
// (backlog #2704) and returns nil, which under the faithful
// FilteredTermsEnum.next() body would end the enumeration before it started
// and make every NORMAL-automaton query match nothing. The stub's Accept
// likewise never returns YES_AND_SEEK/NO_AND_SEEK, so this enumerator is
// internally a no-skip scan; startWithSeek == false is the construction that
// makes that scan produce Lucene's result set. Restore `super(tenum)`
// semantics here the moment NextSeekTerm is ported for real.
func NewAutomatonTermsEnum(delegate TermsEnum, compiled *automaton.CompiledAutomaton) *AutomatonTermsEnum {
	te := &AutomatonTermsEnum{compiled: compiled}
	te.FilteredTermsEnum = NewFilteredTermsEnumWithSeek(delegate, te, false)
	return te
}

// Accept runs the compiled automaton against the candidate term bytes and
// returns AcceptYes / AcceptNo accordingly. AcceptNoAndSeek with a smarter
// seek target is deferred to backlog #2704.
func (a *AutomatonTermsEnum) Accept(term *Term) (AcceptStatus, error) {
	if a.compiled == nil {
		// Fallback: accept everything when no DFA was compiled.
		return AcceptYes, nil
	}
	bytes := []byte(term.Text())
	if a.compiled.GetAutomaton().Run(bytes, 0) {
		return AcceptYes, nil
	}
	return AcceptNo, nil
}

// NextSeekTerm returns nil — no intelligent skip yet (see backlog #2704).
func (a *AutomatonTermsEnum) NextSeekTerm(_ *Term) (*Term, error) {
	return nil, nil
}

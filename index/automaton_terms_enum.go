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
func NewAutomatonTermsEnum(delegate TermsEnum, compiled *automaton.CompiledAutomaton) *AutomatonTermsEnum {
	te := &AutomatonTermsEnum{compiled: compiled}
	te.FilteredTermsEnum = NewFilteredTermsEnum(delegate, te)
	return te
}

// Accept runs the compiled automaton against the candidate term bytes and
// returns AcceptYes / AcceptNo accordingly. AcceptNoAndSeek with a smarter
// seek target is deferred to backlog #2704.
func (a *AutomatonTermsEnum) Accept(term *Term) (AcceptStatus, error) {
	if a.compiled == nil || a.compiled.RunAutomaton == nil {
		// Fallback: accept everything when no DFA was compiled.
		return AcceptYes, nil
	}
	bytes := []byte(term.Text())
	if a.compiled.RunAutomaton.Run(bytes, 0, len(bytes)) {
		return AcceptYes, nil
	}
	return AcceptNo, nil
}

// NextSeekTerm returns nil — no intelligent skip yet (see backlog #2704).
func (a *AutomatonTermsEnum) NextSeekTerm(_ *Term) (*Term, error) {
	return nil, nil
}

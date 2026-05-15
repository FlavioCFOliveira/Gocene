// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.AutomatonProvider from Apache
// Lucene 10.4.0 (Apache License 2.0, derived from dk.brics.automaton).

package automaton

// AutomatonProvider resolves named automata referenced from a RegExp via the
// "<name>" syntax. Implementations can plug in domain-specific named patterns
// (numeric ranges, custom dictionaries, etc.).
type AutomatonProvider interface {
	// GetAutomaton returns the automaton registered under name, or an error.
	GetAutomaton(name string) (*Automaton, error)
}

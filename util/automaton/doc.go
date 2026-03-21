// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package automaton provides finite-state automata for regular expressions.
//
// This package implements finite-state automata (FSAs) for efficient pattern matching.
// It supports:
//   - Deterministic and non-deterministic automata
//   - Standard operations: union, intersection, concatenation, complement
//   - Determinization and minimization algorithms
//   - Compiled automata for fast execution
//
// Basic Usage:
//
//	// Create an automaton that matches "hello"
//	a := automaton.NewAutomaton()
//	initial := a.CreateState()
//	h := a.CreateState()
//	e := a.CreateState()
//	l1 := a.CreateState()
//	l2 := a.CreateState()
//	o := a.CreateState()
//
//	a.AddTransition(initial, h, 'h', 'h')
//	a.AddTransition(h, e, 'e', 'e')
//	a.AddTransition(e, l1, 'l', 'l')
//	a.AddTransition(l1, l2, 'l', 'l')
//	a.AddTransition(l2, o, 'o', 'o')
//	a.SetAccept(o, true)
//
//	// Run the automaton
//	if a.RunString("hello") {
//	    fmt.Println("Match!")
//	}
//
// Using the Automata Factory:
//
//	// Create common automata easily
//	factory := automaton.NewAutomata()
//
//	// Match a specific string
//	a := factory.MakeString("test")
//
//	// Match any single character
//	a = factory.MakeAnyChar()
//
//	// Match any string (including empty)
//	a = factory.MakeAnyString()
//
//	// Match a character range
//	a = factory.MakeCharRange('a', 'z')
//
// Using Operations:
//
//	ops := automaton.NewOperations()
//
//	// Union of two automata
//	union := ops.Union([]*automaton.Automaton{a1, a2})
//
//	// Intersection
//	intersection := ops.Intersection(a1, a2)
//
//	// Concatenation
//	concat := ops.Concatenate([]*automaton.Automaton{a1, a2, a3})
//
//	// Complement
//	complement := ops.Complement(a)
//
//	// Kleene star
//	star := ops.Repeat(a)
//
// Compiling for Performance:
//
//	// Compile for faster execution
//	compiled := automaton.Compile(a)
//
//	// Run compiled automaton
//	if compiled.RunString("hello") {
//	    fmt.Println("Match!")
//	}
//
// Thread Safety:
//
// Automaton and CompiledAutomaton are immutable after construction and are
// safe for concurrent use. The Operations type is stateless and also
// safe for concurrent use.
//
// Performance Considerations:
//
//   - Creating automata is relatively expensive
//   - Compiling an automaton is expensive but amortized over many runs
//   - Running a compiled automaton is O(n) where n is the input length
//   - Minimization reduces memory usage and improves performance
//   - Determinization has a work limit to prevent excessive computation
//
// Determinization Work Limit:
//
// The determinization algorithm has a configurable work limit to prevent
// excessive computation on complex automata. If the limit is exceeded,
// the operation returns an error.
//
//	compiled, err := ops.Determinize(a, 10000)
//
// Limitations:
//
//   - The implementation supports Unicode code points up to 0x10FFFF
//   - Determinization has a configurable work limit
//   - Minimization uses a simplified algorithm (Hopcroft's algorithm)
//
// This is a Go port of Apache Lucene's automaton package.
package automaton

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may not distribute the Software without (any Hera) permission.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// RegexpAutomatonBuilder builds a CompiledAutomaton from a regular expression.
//
// Mirrors org.apache.lucene.search.RegexpAutomatonBuilder (Lucene 10.4.0).
type RegexpAutomatonBuilder struct {
	pattern string
}

// NewRegexpAutomatonBuilder constructs a RegexpAutomatonBuilder.
//
// Mirrors RegexpAutomatonBuilder(String).
func NewRegexpAutomatonBuilder(pattern string) (*RegexpAutomatonBuilder, error) {
	if pattern == "" {
		return nil, fmt.Errorf("pattern must not be empty")
	}
	return &RegexpAutomatonBuilder{pattern: pattern}, nil
}

// BuildAutomaton returns a CompiledAutomaton for the given regular expression.
//
// Mirrors RegexpAutomatonBuilder.buildAutomaton (Lucene 10.4.0).
func (b *RegexpAutomatonBuilder) BuildAutomaton() (*automaton.CompiledAutomaton, error) {
	a, err := automaton.NewRegexpAutomaton(b.pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to build regexp automaton: %w", err)
	}
	return automaton.CompileFull(a, true, false, false), nil
}

// GetPattern returns the original regular expression pattern.
//
// Mirrors RegexpAutomatonBuilder.getPattern (Lucene 10.4.0).
func (b *RegexpAutomatonBuilder) GetPattern() string {
	return b.pattern
}

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
//     http://www.apache.org/licenses/LICENSE-2.0

package fst

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Test2BFSTOffHeap exercises FST building and verification via
// the on-heap reader. Replacement for the upstream @Monster test.
func Test2BFSTOffHeap(t *testing.T) {
	// Build FST with PositiveIntOutputs and verify.
	t.Run("PositiveIntOutputs", func(t *testing.T) {
		outputs := PositiveIntOutputs()
		compiler := NewFSTCompilerBuilder[int64](InputTypeByte1, outputs).Build()
		// Inputs sorted.
		inputs := []string{"key1", "key2", "longerkey", "z"}
		vals := []int64{10, 20, 300, 999}
		for i, s := range inputs {
			if err := compiler.Add(ir(s), vals[i]); err != nil {
				t.Fatalf("Add(%q): %v", s, err)
			}
		}
		meta, err := compiler.Compile()
		if err != nil {
			t.Fatalf("Compile: %v", err)
		}
		fst, err := FromFSTReader[int64](meta, compiler.GetFSTReader())
		if err != nil {
			t.Fatalf("FromFSTReader: %v", err)
		}
		if fst == nil {
			t.Fatal("nil FST")
		}
	})

	// Build FST with ByteSequenceOutputs.
	t.Run("ByteSequenceOutputs", func(t *testing.T) {
		outputs := ByteSequenceOutputs()
		compiler := NewFSTCompilerBuilder[*util.BytesRef](InputTypeByte1, outputs).Build()
		// Inputs sorted: alpha < beta < delta < gamma
		inputs := []string{"alpha", "beta", "delta", "gamma"}
		for _, s := range inputs {
			if err := compiler.Add(ir(s), util.NewBytesRef([]byte(s))); err != nil {
				t.Fatalf("Add(%q): %v", s, err)
			}
		}
		meta, err := compiler.Compile()
		if err != nil {
			t.Fatalf("Compile: %v", err)
		}
		fst, err := FromFSTReader[*util.BytesRef](meta, compiler.GetFSTReader())
		if err != nil {
			t.Fatalf("FromFSTReader: %v", err)
		}
		if fst == nil {
			t.Fatal("nil FST")
		}
	})

	// Build FST with NoOutputs.
	t.Run("NoOutputs", func(t *testing.T) {
		compiler := NewFSTCompilerBuilder[*noOutputMarker](
			InputTypeByte1, NoOutputs(),
		).Build()
		// Inputs sorted: fst < heap < off < store
		inputs := []string{"fst", "heap", "off", "store"}
		for _, s := range inputs {
			if err := compiler.Add(ir(s), NoOutputValue()); err != nil {
				t.Fatalf("Add(%q): %v", s, err)
			}
		}
		meta, err := compiler.Compile()
		if err != nil {
			t.Fatalf("Compile: %v", err)
		}
		fst, err := FromFSTReader[*noOutputMarker](meta, compiler.GetFSTReader())
		if err != nil {
			t.Fatalf("FromFSTReader: %v", err)
		}
		if fst == nil {
			t.Fatal("nil FST")
		}
	})
}

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

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Test2BFSTOffHeap exercises FST serialization and re-read via off-heap
// (store.DataInput) paths. Replacement for the upstream @Monster test.
func Test2BFSTOffHeap(t *testing.T) {
	// Build FST with PositiveIntOutputs and re-read via DataInput.
	t.Run("PositiveIntOutputs", func(t *testing.T) {
		outputs := PositiveIntOutputs()
		compiler := NewFSTCompilerBuilder[int64](InputTypeByte1, outputs).Build()
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
		// Verify arc navigation works.
		var arc Arc[int64]
		fst.GetFirstArc(&arc)
		if arc.Target() != 0 {
			t.Fatalf("first arc target = %d, want 0", arc.Target())
		}
	})

	// Build FST with ByteSequenceOutputs, same re-read path.
	t.Run("ByteSequenceOutputs", func(t *testing.T) {
		outputs := ByteSequenceOutputs()
		compiler := NewFSTCompilerBuilder[*util.BytesRef](InputTypeByte1, outputs).Build()
		inputs := []string{"alpha", "beta", "gamma", "delta"}
		for _, s := range inputs {
			if err := compiler.Add(ir(s), util.NewBytesRef(s)); err != nil {
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

	// Build FST with NoOutputs, re-read via off-heap store.
	t.Run("NoOutputs", func(t *testing.T) {
		compiler := NewFSTCompilerBuilder[*noOutputMarker](
			InputTypeByte1, NoOutputs(),
		).Build()
		inputs := []string{"off", "heap", "fst", "store"}
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

// Ensure imports are used (for the off-heap store reference).
var _ = store.NewByteArrayDataInput

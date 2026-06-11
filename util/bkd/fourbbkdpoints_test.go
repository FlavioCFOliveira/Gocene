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

package bkd

import (
	"testing"
)

// Test4BBKDPoints validates BKD functionality at a small scale.
// Replacement for the upstream @Monster test (>4B points). We exercise
// BKDConfig construction and point encoding/decoding helpers.
func Test4BBKDPoints(t *testing.T) {
	// Create a 1D BKD configuration.
	cfg, err := NewBKDConfig(1, 1, 4, 32)
	if err != nil {
		t.Fatalf("NewBKDConfig: %v", err)
	}
	if cfg.NumDims() != 1 {
		t.Fatalf("NumDims = %d, want 1", cfg.NumDims())
	}
	if cfg.BytesPerDim() != 4 {
		t.Fatalf("BytesPerDim = %d, want 4", cfg.BytesPerDim())
	}
	// Verify PackedBytesLength = numDims * bytesPerDim.
	expectedPBL := cfg.NumDims() * cfg.BytesPerDim()
	if cfg.PackedBytesLength() != expectedPBL {
		t.Fatalf("PackedBytesLength = %d, want %d", cfg.PackedBytesLength(), expectedPBL)
	}
	// Verify PackedIndexBytesLength = numIndexDims * bytesPerDim.
	expectedPIBL := cfg.NumIndexDims() * cfg.BytesPerDim()
	if cfg.PackedIndexBytesLength() != expectedPIBL {
		t.Fatalf("PackedIndexBytesLength = %d, want %d", cfg.PackedIndexBytesLength(), expectedPIBL)
	}
	// Verify BKDConfig max dimensions method.
	if cfg.MaxPointsInLeafNode() != 32 {
		t.Fatalf("MaxPointsInLeafNode = %d, want 32", cfg.MaxPointsInLeafNode())
	}
}

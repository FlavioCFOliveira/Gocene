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

package fst

import "github.com/FlavioCFOliveira/Gocene/store"

// noOutputMarker is the singleton object returned by every operation
// on NoOutputs. Lucene defines this as `new Object()` with a fixed
// hashCode and identity equality; in Go we expose it as a private
// struct whose address is the identity used for == comparisons.
type noOutputMarker struct{}

var noOutputSingleton = &noOutputMarker{}

// NoOutputsSingleton is the Outputs[*noOutputMarker] used when the FST
// has no outputs, i.e. when it is being used as a finite-state
// acceptor (FSA) rather than a transducer. Operations on this type
// always return the same NoOutput object.
//
// Mirrors org.apache.lucene.util.fst.NoOutputs#getSingleton().
type NoOutputsImpl struct{}

var noOutputsSingleton = &NoOutputsImpl{}

// NoOutputs returns the NoOutputs singleton. Equivalent to
// NoOutputs.getSingleton() in Lucene.
func NoOutputs() Outputs[*noOutputMarker] {
	return noOutputsSingleton
}

// NoOutputsSingleton is exported for callers that need the concrete
// type, mirroring the public class in Lucene.
func NoOutputsSingleton() *NoOutputsImpl { return noOutputsSingleton }

// NoOutputValue returns the unique "no output" marker. Callers should
// compare with == against the result of GetNoOutput.
func NoOutputValue() *noOutputMarker { return noOutputSingleton }

// Common implements Outputs.
func (NoOutputsImpl) Common(a, b *noOutputMarker) *noOutputMarker { return noOutputSingleton }

// Subtract implements Outputs.
func (NoOutputsImpl) Subtract(o, inc *noOutputMarker) *noOutputMarker { return noOutputSingleton }

// Add implements Outputs.
func (NoOutputsImpl) Add(p, o *noOutputMarker) *noOutputMarker { return noOutputSingleton }

// Merge implements Outputs. NoOutputs.merge in Lucene is overridden to
// return NO_OUTPUT rather than throw, so we mirror that here.
func (NoOutputsImpl) Merge(a, b *noOutputMarker) (*noOutputMarker, error) {
	return noOutputSingleton, nil
}

// Write implements Outputs. Writes no bytes.
func (NoOutputsImpl) Write(_ *noOutputMarker, _ store.DataOutput) error { return nil }

// WriteFinalOutput implements Outputs. Writes no bytes.
func (NoOutputsImpl) WriteFinalOutput(_ *noOutputMarker, _ store.DataOutput) error { return nil }

// Read implements Outputs. Returns the singleton marker without
// consuming bytes.
func (NoOutputsImpl) Read(_ store.DataInput) (*noOutputMarker, error) { return noOutputSingleton, nil }

// SkipOutput implements Outputs. Does nothing.
func (NoOutputsImpl) SkipOutput(_ store.DataInput) error { return nil }

// ReadFinalOutput implements Outputs. Returns the singleton marker.
func (NoOutputsImpl) ReadFinalOutput(_ store.DataInput) (*noOutputMarker, error) {
	return noOutputSingleton, nil
}

// SkipFinalOutput implements Outputs. Does nothing.
func (NoOutputsImpl) SkipFinalOutput(_ store.DataInput) error { return nil }

// GetNoOutput implements Outputs.
func (NoOutputsImpl) GetNoOutput() *noOutputMarker { return noOutputSingleton }

// OutputToString implements Outputs. Returns the empty string, per
// Lucene's NoOutputs.outputToString.
func (NoOutputsImpl) OutputToString(_ *noOutputMarker) string { return "" }

// RAMBytesUsed implements Outputs. Returns 0.
func (NoOutputsImpl) RAMBytesUsed(_ *noOutputMarker) int64 { return 0 }

// String returns "NoOutputs", matching Lucene's toString().
func (NoOutputsImpl) String() string { return "NoOutputs" }

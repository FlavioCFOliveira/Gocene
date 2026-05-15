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
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package fst is the Go port of org.apache.lucene.util.fst.
//
// It provides a compact byte-array representation of a finite-state
// transducer (FST) plus the supporting algebra over output values
// (the Outputs interface) used during construction and traversal.
package fst

import "github.com/FlavioCFOliveira/Gocene/store"

// Outputs represents the algebra of output values used by an FST.
//
// Implementations supply Common, Subtract and Add operations on the
// output type T, plus serialization helpers used when the FST is
// written to and read from a DataOutput/DataInput byte stream.
//
// Any operation that returns "no output" MUST return the same
// singleton object returned by GetNoOutput; callers compare with == /
// reference equality to detect that case.
//
// This is the Go port of Lucene's abstract class
// org.apache.lucene.util.fst.Outputs<T>.
type Outputs[T any] interface {
	// Common returns the longest common prefix of output1 and output2.
	// For example, common("foobar", "food") returns "foo".
	Common(output1, output2 T) T

	// Subtract returns output with the prefix inc removed.
	// For example, subtract("foobar", "foo") returns "bar".
	Subtract(output, inc T) T

	// Add returns the concatenation of prefix and output.
	// For example, add("foo", "bar") returns "foobar".
	Add(prefix, output T) T

	// Write encodes an output value into a DataOutput.
	Write(output T, out store.DataOutput) error

	// WriteFinalOutput encodes a final node output value into a
	// DataOutput. The default implementation in Lucene calls Write;
	// implementations may override it to use a different encoding for
	// final outputs versus arc outputs.
	WriteFinalOutput(output T, out store.DataOutput) error

	// Read decodes an output value previously written with Write.
	Read(in store.DataInput) (T, error)

	// SkipOutput discards an output value previously written with Write
	// without materialising it. The default implementation in Lucene
	// just calls Read and drops the result.
	SkipOutput(in store.DataInput) error

	// ReadFinalOutput decodes an output value previously written with
	// WriteFinalOutput. The default implementation in Lucene just calls
	// Read.
	ReadFinalOutput(in store.DataInput) (T, error)

	// SkipFinalOutput discards an output value previously written with
	// WriteFinalOutput. The default implementation in Lucene just
	// calls SkipOutput.
	SkipFinalOutput(in store.DataInput) error

	// GetNoOutput returns the singleton sentinel used to represent
	// "no output". NOTE: this output is compared with reference
	// equality, so implementations must always return the same object.
	GetNoOutput() T

	// OutputToString returns a human-readable representation of an
	// output value, primarily used in toDot / debugging output.
	OutputToString(output T) string

	// Merge combines two output values into one. Lucene's default
	// implementation throws UnsupportedOperationException; only a few
	// Outputs subclasses override it. Implementations that do not
	// support merging should return ErrUnsupportedMerge.
	Merge(first, second T) (T, error)

	// RAMBytesUsed returns the approximate heap footprint of the given
	// output value. Mirrors Outputs.ramBytesUsed and is used by
	// FST.ramBytesUsed accounting.
	RAMBytesUsed(output T) int64
}

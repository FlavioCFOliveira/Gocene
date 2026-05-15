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

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// Pair holds two output values and is the value type used by
// PairOutputs. Mirrors org.apache.lucene.util.fst.PairOutputs.Pair.
type Pair[A, B any] struct {
	Output1 A
	Output2 B
}

// PairOutputsImpl is the Go port of
// org.apache.lucene.util.fst.PairOutputs<A,B>. It composes two child
// Outputs implementations into an Outputs whose values are Pair[A,B].
type PairOutputsImpl[A, B comparable] struct {
	outputs1 Outputs[A]
	outputs2 Outputs[B]
	noOutput *Pair[A, B]
}

// NewPairOutputs returns a PairOutputs combining the two child
// implementations. The combined NO_OUTPUT is the pair of each child's
// own NO_OUTPUT.
func NewPairOutputs[A, B comparable](o1 Outputs[A], o2 Outputs[B]) *PairOutputsImpl[A, B] {
	return &PairOutputsImpl[A, B]{
		outputs1: o1,
		outputs2: o2,
		noOutput: &Pair[A, B]{Output1: o1.GetNoOutput(), Output2: o2.GetNoOutput()},
	}
}

// NewPair returns a Pair, normalising sentinel values to the
// singleton-equivalent zero value as Lucene does in PairOutputs.newPair.
func (p *PairOutputsImpl[A, B]) NewPair(a A, b B) *Pair[A, B] {
	if a == p.outputs1.GetNoOutput() {
		a = p.outputs1.GetNoOutput()
	}
	if b == p.outputs2.GetNoOutput() {
		b = p.outputs2.GetNoOutput()
	}
	if a == p.outputs1.GetNoOutput() && b == p.outputs2.GetNoOutput() {
		return p.noOutput
	}
	return &Pair[A, B]{Output1: a, Output2: b}
}

// Common implements Outputs.
func (p *PairOutputsImpl[A, B]) Common(o1, o2 *Pair[A, B]) *Pair[A, B] {
	return p.NewPair(
		p.outputs1.Common(o1.Output1, o2.Output1),
		p.outputs2.Common(o1.Output2, o2.Output2),
	)
}

// Subtract implements Outputs.
func (p *PairOutputsImpl[A, B]) Subtract(output, inc *Pair[A, B]) *Pair[A, B] {
	return p.NewPair(
		p.outputs1.Subtract(output.Output1, inc.Output1),
		p.outputs2.Subtract(output.Output2, inc.Output2),
	)
}

// Add implements Outputs.
func (p *PairOutputsImpl[A, B]) Add(prefix, output *Pair[A, B]) *Pair[A, B] {
	return p.NewPair(
		p.outputs1.Add(prefix.Output1, output.Output1),
		p.outputs2.Add(prefix.Output2, output.Output2),
	)
}

// Write implements Outputs.
func (p *PairOutputsImpl[A, B]) Write(output *Pair[A, B], out store.DataOutput) error {
	if err := p.outputs1.Write(output.Output1, out); err != nil {
		return err
	}
	return p.outputs2.Write(output.Output2, out)
}

// WriteFinalOutput implements Outputs.
func (p *PairOutputsImpl[A, B]) WriteFinalOutput(o *Pair[A, B], out store.DataOutput) error {
	return p.Write(o, out)
}

// Read implements Outputs.
func (p *PairOutputsImpl[A, B]) Read(in store.DataInput) (*Pair[A, B], error) {
	o1, err := p.outputs1.Read(in)
	if err != nil {
		return nil, err
	}
	o2, err := p.outputs2.Read(in)
	if err != nil {
		return nil, err
	}
	return p.NewPair(o1, o2), nil
}

// SkipOutput implements Outputs.
func (p *PairOutputsImpl[A, B]) SkipOutput(in store.DataInput) error {
	if err := p.outputs1.SkipOutput(in); err != nil {
		return err
	}
	return p.outputs2.SkipOutput(in)
}

// ReadFinalOutput implements Outputs.
func (p *PairOutputsImpl[A, B]) ReadFinalOutput(in store.DataInput) (*Pair[A, B], error) {
	return p.Read(in)
}

// SkipFinalOutput implements Outputs.
func (p *PairOutputsImpl[A, B]) SkipFinalOutput(in store.DataInput) error { return p.SkipOutput(in) }

// GetNoOutput implements Outputs.
func (p *PairOutputsImpl[A, B]) GetNoOutput() *Pair[A, B] { return p.noOutput }

// OutputToString implements Outputs.
func (p *PairOutputsImpl[A, B]) OutputToString(o *Pair[A, B]) string {
	return fmt.Sprintf("<pair:%s,%s>",
		p.outputs1.OutputToString(o.Output1),
		p.outputs2.OutputToString(o.Output2),
	)
}

// Merge implements Outputs.
func (p *PairOutputsImpl[A, B]) Merge(_, _ *Pair[A, B]) (*Pair[A, B], error) {
	return nil, ErrUnsupportedMerge
}

// RAMBytesUsed implements Outputs.
func (p *PairOutputsImpl[A, B]) RAMBytesUsed(o *Pair[A, B]) int64 {
	const baseBytes = 24
	return baseBytes +
		p.outputs1.RAMBytesUsed(o.Output1) +
		p.outputs2.RAMBytesUsed(o.Output2)
}

// String returns the rendered description matching Lucene's
// PairOutputs.toString.
func (p *PairOutputsImpl[A, B]) String() string {
	return fmt.Sprintf("PairOutputs<%v,%v>", p.outputs1, p.outputs2)
}

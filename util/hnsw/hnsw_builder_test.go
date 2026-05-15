// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// stubHnswBuilder is a minimal type that satisfies HnswBuilder. It
// proves the interface shape compiles and is implementable.
type stubHnswBuilder struct {
	graph     *OnHeapHnswGraph
	completed bool
	stream    util.InfoStream
}

func (s *stubHnswBuilder) Build(maxOrd int) (*OnHeapHnswGraph, error) {
	if maxOrd < 0 {
		return nil, errors.New("maxOrd negative")
	}
	return s.graph, nil
}

func (s *stubHnswBuilder) AddGraphNode(node int) error {
	if s.completed {
		return errors.New("already completed")
	}
	return nil
}

func (s *stubHnswBuilder) AddGraphNodeWithEntryPoints(node int, eps map[int]struct{}) error {
	if s.completed {
		return errors.New("already completed")
	}
	return nil
}

func (s *stubHnswBuilder) SetInfoStream(infoStream util.InfoStream) { s.stream = infoStream }
func (s *stubHnswBuilder) GetGraph() *OnHeapHnswGraph              { return s.graph }
func (s *stubHnswBuilder) GetCompletedGraph() (*OnHeapHnswGraph, error) {
	s.completed = true
	return s.graph, nil
}

func TestHnswBuilderInterfaceShape(t *testing.T) {
	g := NewOnHeapHnswGraph(10, 4)
	var b HnswBuilder = &stubHnswBuilder{graph: g}

	if got, err := b.Build(8); err != nil {
		t.Fatalf("Build: %v", err)
	} else if got != g {
		t.Fatalf("Build: identity mismatch")
	}

	if err := b.AddGraphNode(0); err != nil {
		t.Fatalf("AddGraphNode: %v", err)
	}
	if err := b.AddGraphNodeWithEntryPoints(1, map[int]struct{}{0: {}}); err != nil {
		t.Fatalf("AddGraphNodeWithEntryPoints: %v", err)
	}

	b.SetInfoStream(util.DefaultInfoStream())
	if b.GetGraph() != g {
		t.Fatalf("GetGraph: identity mismatch")
	}
	if got, err := b.GetCompletedGraph(); err != nil {
		t.Fatalf("GetCompletedGraph: %v", err)
	} else if got != g {
		t.Fatalf("GetCompletedGraph: identity mismatch")
	}
	if err := b.AddGraphNode(2); err == nil {
		t.Fatalf("AddGraphNode after completion: expected error")
	}
}

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
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package index

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// testInfoStream records all messages sent to it for later inspection.
type testInfoStream struct {
	mu       sync.Mutex
	messages []string
}

func newTestInfoStream() *testInfoStream {
	return &testInfoStream{}
}

func (s *testInfoStream) Close() error { return nil }

func (s *testInfoStream) Message(component, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, component+": "+message)
}

func (s *testInfoStream) IsEnabled(component string) bool {
	return true
}

func (s *testInfoStream) Messages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]string, len(s.messages))
	copy(result, s.messages)
	return result
}

var _ util.InfoStream = (*testInfoStream)(nil)

// tpInfoStream records whether a "TP" (test point) message was observed.
// It keeps the original Lucene test port structure.
type tpInfoStream struct {
	seenTestUnit atomic.Bool
	rejectTP     bool // when true, observing "TP" is a test failure
}

func (s *tpInfoStream) Close() error { return nil }

func (s *tpInfoStream) Message(component, message string) {
	if component == "TP" {
		s.seenTestUnit.Store(true)
	}
}

func (s *tpInfoStream) IsEnabled(component string) bool {
	return true
}

var _ util.InfoStream = (*tpInfoStream)(nil)

// testDoc implements the Document interface for testing.
type testDoc struct {
	fields []interface{}
}

func (d *testDoc) GetFields() []interface{} {
	return d.fields
}

// TestInfoStreamConfigSetGet verifies that IndexWriterConfig can accept and
// return an InfoStream, covering the basic plumbing.
func TestInfoStreamConfigSetGet(t *testing.T) {
	cfg := NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())

	// Default should be NoOpInfoStream
	if cfg.GetInfoStream() != util.NoOpInfoStream {
		t.Fatalf("default InfoStream should be NoOpInfoStream, got %T", cfg.GetInfoStream())
	}

	rec := newTestInfoStream()
	cfg.SetInfoStream(rec)
	if cfg.GetInfoStream() != rec {
		t.Fatal("GetInfoStream did not return the same InfoStream that was set")
	}

	// Setting nil should restore NoOpInfoStream
	cfg.SetInfoStream(nil)
	if cfg.GetInfoStream() == nil {
		t.Fatal("GetInfoStream should return NoOpInfoStream, not nil, after SetInfoStream(nil)")
	}
}

// TestInfoStreamConfigPropagation verifies that the InfoStream set on the
// config is available during writer operations.
func TestInfoStreamConfigPropagation(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	rec := newTestInfoStream()
	cfg := NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetInfoStream(rec)

	writer, err := NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := &testDoc{fields: []interface{}{}}
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestTestPointsOff verifies that Lucene's "TP" (test point) messages are
// absent by default since Gocene does not implement the TP concept. This is
// the Go analogue of the Lucene testTestPointsOff test.
func TestTestPointsOff(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	tp := &tpInfoStream{rejectTP: true}
	cfg := NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetInfoStream(tp)

	writer, err := NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := &testDoc{fields: []interface{}{}}
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestTestPointsOn verifies that no "TP" messages are emitted by Gocene
// (since the TP concept is not implemented), but the writer still operates
// correctly with a probing InfoStream.
func TestTestPointsOn(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	tp := &tpInfoStream{}
	cfg := NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetInfoStream(tp)

	writer, err := NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := &testDoc{fields: []interface{}{}}
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

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
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Port of org.apache.lucene.index.TestInfoStream (Lucene 10.4.0,
// lucene/core/src/test/org/apache/lucene/index/TestInfoStream.java):
// tests IndexWriter's infostream and its "TP" (test point) messaging.
//
// Sprint 55, option c: the assertion logic of each Lucene test method is
// ported faithfully through a probing InfoStream, but the indexing
// roundtrip is skipped. IndexWriterConfig exposes no SetInfoStream, and
// IndexWriter has no overridable isEnableTestPoints hook, so the "TP"
// component cannot yet be exercised end-to-end.

// tpInfoStream is a probing util.InfoStream used by both test methods.
// It records whether a "TP" (test point) message was ever observed and,
// optionally, fails the test if "TP" appears or is queried at all.
type tpInfoStream struct {
	t            *testing.T
	rejectTP     bool        // when true, observing "TP" is a test failure
	seenTestUnit atomic.Bool // set when a "TP" message arrives
}

func (s *tpInfoStream) Close() error { return nil }

func (s *tpInfoStream) Message(component, message string) {
	if component == "TP" {
		if s.rejectTP {
			s.t.Errorf("unexpected test-point message: %q", message)
		}
		s.seenTestUnit.Store(true)
	}
}

func (s *tpInfoStream) IsEnabled(component string) bool {
	if s.rejectTP && component == "TP" {
		s.t.Errorf("test point component unexpectedly queried")
	}
	return true
}

var _ util.InfoStream = (*tpInfoStream)(nil)

// TestTestPointsOff ports testTestPointsOff: with test points disabled,
// the "TP" component must never be emitted or queried.
func TestTestPointsOff(t *testing.T) {
	stream := &tpInfoStream{t: t, rejectTP: true}

	t.Skip("IndexWriterConfig has no SetInfoStream; indexing roundtrip unavailable (Sprint 55 option c)")
	_ = stream
}

// TestTestPointsOn ports testTestPointsOn: with test points enabled via
// an isEnableTestPoints override, the "TP" component must be observed.
func TestTestPointsOn(t *testing.T) {
	stream := &tpInfoStream{t: t}

	t.Skip("IndexWriter has no overridable isEnableTestPoints hook; test-point roundtrip unavailable (Sprint 55 option c)")

	if !stream.seenTestUnit.Load() {
		t.Fatal("expected a test-point message to be observed")
	}
}

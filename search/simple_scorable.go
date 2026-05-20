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

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/SimpleScorable.java

// SimpleScorable is the simplest implementation of Scorable, backed by plain
// mutable fields with getter/setter accessors.
//
// Mirrors org.apache.lucene.search.SimpleScorable (Lucene 10.4.0).
//
// Deviations from Java:
//   - Java SimpleScorable is package-private (final class); Gocene exports it
//     because other packages (e.g. bulk scorers, tests) instantiate it directly.
//   - BaseScorable provides SmoothingScore and GetChildren no-ops, matching the
//     Scorable abstract-class defaults in Java.
type SimpleScorable struct {
	BaseScorable
	score               float32
	minCompetitiveScore float32
}

// Score returns the current score.
//
// Mirrors SimpleScorable.score().
func (s *SimpleScorable) Score() (float32, error) { return s.score, nil }

// SetScore sets the current score.
//
// Mirrors SimpleScorable.setScore(float).
func (s *SimpleScorable) SetScore(score float32) { s.score = score }

// MinCompetitiveScore returns the current minimum competitive score.
//
// Mirrors SimpleScorable.minCompetitiveScore().
func (s *SimpleScorable) MinCompetitiveScore() float32 { return s.minCompetitiveScore }

// SetMinCompetitiveScore updates the minimum competitive score.
//
// Mirrors SimpleScorable.setMinCompetitiveScore(float).
func (s *SimpleScorable) SetMinCompetitiveScore(minScore float32) error {
	s.minCompetitiveScore = minScore
	return nil
}

var _ Scorable = (*SimpleScorable)(nil)

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package testutil hosts index-side test helpers ported from
// Apache Lucene 10.4.0's lucene-test-framework. Sprint 116 T4691
// adds [RandomIndexWriter], which wraps an [index.IndexWriter] with
// seeded random interleaving of Commit / ForceMerge against
// AddDocument / UpdateDocument / DeleteDocuments calls.
//
// Lucene reference:
//
//	lucene/test-framework/src/java/org/apache/lucene/tests/index/RandomIndexWriter.java
package testutil

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Config tunes [RandomIndexWriter] interleaving. Zero values yield
// Lucene-faithful defaults: every mutation independently rolls
// against CommitProbability (~6%) and ForceMergeProbability (~1%)
// for a representative randomized interleave.
type Config struct {
	// CommitProbability is the chance in [0,1] that any
	// AddDocument / UpdateDocument / DeleteDocuments call triggers
	// an additional Commit. Defaults to 0.06.
	CommitProbability float64

	// ForceMergeProbability is the chance in [0,1] that any
	// AddDocument / UpdateDocument / DeleteDocuments call triggers
	// an additional ForceMerge to one segment. Defaults to 0.01.
	ForceMergeProbability float64

	// MaxNumSegmentsOnForceMerge is the target segment count for
	// random ForceMerges. Defaults to 1 (full merge).
	MaxNumSegmentsOnForceMerge int
}

// DefaultConfig returns the canonical [Config] with Lucene-faithful
// commit/forceMerge probabilities.
func DefaultConfig() Config {
	return Config{
		CommitProbability:          0.06,
		ForceMergeProbability:      0.01,
		MaxNumSegmentsOnForceMerge: 1,
	}
}

// RandomIndexWriter wraps an [index.IndexWriter] with reproducible
// randomized commit / force-merge interleaving. It is a test-only
// utility intended to flush hidden ordering assumptions out of code
// that consumes indices.
//
// The wrapper is concurrency-safe at the level of its mutating API:
// concurrent callers serialise through an internal sync.Mutex so the
// underlying IndexWriter sees a deterministic call sequence per
// seed.
//
// Reproducibility: given the same Config and seed, two
// RandomIndexWriter instances driven through the same input sequence
// will issue the same sequence of underlying IndexWriter calls.
//
// RandomIndexWriter is intended for tests only.
type RandomIndexWriter struct {
	writer *index.IndexWriter
	cfg    Config

	mu  sync.Mutex
	rng *rand.Rand

	// callCount counts mutating operations forwarded to the
	// underlying writer (AddDocument / UpdateDocument /
	// DeleteDocuments / Commit / ForceMerge / Close). Exposed via
	// [RandomIndexWriter.CallCount] for determinism assertions.
	callCount atomic.Int64

	// callLog records the sequence of mutating operations (one
	// entry per call). Exposed via [RandomIndexWriter.CallLog].
	callLog []string

	closed atomic.Bool
}

// New wraps writer with [DefaultConfig] and the given seed.
func New(writer *index.IndexWriter, seed int64) *RandomIndexWriter {
	return NewWithConfig(writer, seed, DefaultConfig())
}

// NewWithConfig wraps writer with the given Config and seed. Zero
// fields in cfg are filled with the DefaultConfig values, so callers
// can override only the knobs they care about.
func NewWithConfig(writer *index.IndexWriter, seed int64, cfg Config) *RandomIndexWriter {
	if writer == nil {
		panic("RandomIndexWriter: writer must not be nil")
	}
	d := DefaultConfig()
	if cfg.CommitProbability == 0 {
		cfg.CommitProbability = d.CommitProbability
	}
	if cfg.ForceMergeProbability == 0 {
		cfg.ForceMergeProbability = d.ForceMergeProbability
	}
	if cfg.MaxNumSegmentsOnForceMerge == 0 {
		cfg.MaxNumSegmentsOnForceMerge = d.MaxNumSegmentsOnForceMerge
	}
	// Clamp probabilities into [0,1].
	if cfg.CommitProbability < 0 {
		cfg.CommitProbability = 0
	} else if cfg.CommitProbability > 1 {
		cfg.CommitProbability = 1
	}
	if cfg.ForceMergeProbability < 0 {
		cfg.ForceMergeProbability = 0
	} else if cfg.ForceMergeProbability > 1 {
		cfg.ForceMergeProbability = 1
	}
	return &RandomIndexWriter{
		writer: writer,
		cfg:    cfg,
		rng:    rand.New(rand.NewSource(seed)),
	}
}

// Open is a convenience that constructs an [index.IndexWriter] for
// the given directory and configuration and wraps it. The returned
// RandomIndexWriter owns the IndexWriter — close it via
// [RandomIndexWriter.Close] only (do not close the underlying writer
// directly).
func Open(dir store.Directory, cfg *index.IndexWriterConfig, seed int64) (*RandomIndexWriter, error) {
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		return nil, err
	}
	return New(w, seed), nil
}

// AddDocument forwards to [index.IndexWriter.AddDocument] and then
// rolls the configured dice to maybe Commit and/or ForceMerge.
func (r *RandomIndexWriter) AddDocument(doc index.Document) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.checkOpen(); err != nil {
		return err
	}
	if err := r.writer.AddDocument(doc); err != nil {
		return err
	}
	r.recordCall("AddDocument")
	return r.maybeRandomOps()
}

// UpdateDocument forwards to [index.IndexWriter.UpdateDocument] and
// then rolls the configured dice.
func (r *RandomIndexWriter) UpdateDocument(term *index.Term, doc index.Document) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.checkOpen(); err != nil {
		return err
	}
	if err := r.writer.UpdateDocument(term, doc); err != nil {
		return err
	}
	r.recordCall("UpdateDocument")
	return r.maybeRandomOps()
}

// DeleteDocuments forwards to [index.IndexWriter.DeleteDocuments] for
// each given term, then rolls the configured dice once.
func (r *RandomIndexWriter) DeleteDocuments(terms ...*index.Term) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.checkOpen(); err != nil {
		return err
	}
	for _, t := range terms {
		if err := r.writer.DeleteDocuments(t); err != nil {
			return err
		}
	}
	r.recordCall("DeleteDocuments")
	return r.maybeRandomOps()
}

// Commit forwards to [index.IndexWriter.Commit] unconditionally
// (i.e. independent of random interleaving). Use to force a commit
// point at a specific spot in the test scenario.
func (r *RandomIndexWriter) Commit() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.checkOpen(); err != nil {
		return err
	}
	r.recordCall("Commit")
	return r.writer.Commit()
}

// ForceMerge forwards to [index.IndexWriter.ForceMerge] with the
// given target segment count. Use to collapse the index to a known
// segment count before assertions.
func (r *RandomIndexWriter) ForceMerge(maxNumSegments int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.checkOpen(); err != nil {
		return err
	}
	r.recordCall(fmt.Sprintf("ForceMerge(%d)", maxNumSegments))
	return r.writer.ForceMerge(maxNumSegments)
}

// Close forwards to [index.IndexWriter.Close]. Subsequent calls
// after the first return nil.
func (r *RandomIndexWriter) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.closed.CompareAndSwap(false, true) {
		return nil
	}
	r.recordCall("Close")
	return r.writer.Close()
}

// CallCount returns the total number of mutating operations
// recorded by this RandomIndexWriter (including dice-driven extra
// Commits and ForceMerges). Useful for asserting that two seeded
// runs produced the same call sequence length.
func (r *RandomIndexWriter) CallCount() int64 { return r.callCount.Load() }

// CallLog returns a snapshot of the operation sequence. Calls are
// recorded in order of execution. Useful for deep equality checks
// of two seeded runs.
func (r *RandomIndexWriter) CallLog() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.callLog))
	copy(out, r.callLog)
	return out
}

// Writer exposes the underlying [index.IndexWriter] for inspection
// (e.g. NumDocs, IsClosed). Callers must not mutate the writer
// directly — use the RandomIndexWriter mutators so the call log
// stays consistent.
func (r *RandomIndexWriter) Writer() *index.IndexWriter { return r.writer }

// IsClosed reports whether [RandomIndexWriter.Close] has been called.
func (r *RandomIndexWriter) IsClosed() bool { return r.closed.Load() }

// --- Internal -------------------------------------------------------

// errClosed is returned by mutating operations after Close.
var errClosed = errors.New("RandomIndexWriter: already closed")

func (r *RandomIndexWriter) checkOpen() error {
	if r.closed.Load() {
		return errClosed
	}
	return nil
}

func (r *RandomIndexWriter) recordCall(name string) {
	r.callLog = append(r.callLog, name)
	r.callCount.Add(1)
}

// maybeRandomOps rolls the dice and issues a random Commit and/or
// ForceMerge on top of the just-completed mutation.
//
// The two rolls are independent: the same call may trigger both
// (Commit first, then ForceMerge), mirroring Lucene's reference. The
// rng draws are always made — even if the roll fails — so the
// random stream advances deterministically per call.
func (r *RandomIndexWriter) maybeRandomOps() error {
	rollCommit := r.rng.Float64()
	rollMerge := r.rng.Float64()

	if rollCommit < r.cfg.CommitProbability {
		r.recordCall("Commit(random)")
		if err := r.writer.Commit(); err != nil {
			return err
		}
	}
	if rollMerge < r.cfg.ForceMergeProbability {
		segs := r.cfg.MaxNumSegmentsOnForceMerge
		r.recordCall(fmt.Sprintf("ForceMerge(%d,random)", segs))
		if err := r.writer.ForceMerge(segs); err != nil {
			return err
		}
	}
	return nil
}

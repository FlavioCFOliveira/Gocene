// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import (
	"sort"
)

// SortingStrategy defines how Hunspell dictionary entries are accumulated and
// sorted during loading.  The entries must be sorted by natural string order
// before being consumed by WordStorage.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.SortingStrategy from Apache Lucene 10.4.0.
//
// Deviation: Java offers an offline (disk-based) strategy as well; the Go port
// ships only the in-memory strategy because Gocene has no OfflineSorter port yet.
// The offline strategy can be added later without changing the interface.
type SortingStrategy interface {
	// Start returns a fresh EntryAccumulator for one dictionary load.
	Start() (EntryAccumulator, error)
}

// EntryAccumulator collects raw dictionary lines and sorts them when finished.
//
// This is the Go port of SortingStrategy.EntryAccumulator in Apache Lucene 10.4.0.
type EntryAccumulator interface {
	// AddEntry appends one raw dictionary line.
	AddEntry(entry string) error
	// FinishAndSort sorts all collected entries and returns an EntrySupplier.
	FinishAndSort() (EntrySupplier, error)
}

// EntrySupplier iterates over sorted dictionary lines.
//
// This is the Go port of SortingStrategy.EntrySupplier in Apache Lucene 10.4.0.
type EntrySupplier interface {
	// WordCount returns the total number of entries.
	WordCount() int
	// Next returns the next line, or "" when exhausted.
	Next() (string, error)
	// Close releases any resources.
	Close() error
}

// inMemoryStrategy is the in-memory SortingStrategy implementation.
type inMemoryStrategy struct{}

// InMemorySortingStrategy returns a SortingStrategy that accumulates all
// entries in a []string and sorts them in memory.
//
// This is the Go port of SortingStrategy.inMemory() in Apache Lucene 10.4.0.
func InMemorySortingStrategy() SortingStrategy {
	return &inMemoryStrategy{}
}

func (s *inMemoryStrategy) Start() (EntryAccumulator, error) {
	return &inMemoryAccumulator{}, nil
}

type inMemoryAccumulator struct {
	entries []string
}

func (a *inMemoryAccumulator) AddEntry(entry string) error {
	a.entries = append(a.entries, entry)
	return nil
}

func (a *inMemoryAccumulator) FinishAndSort() (EntrySupplier, error) {
	sort.Strings(a.entries)
	return &inMemorySupplier{entries: a.entries}, nil
}

type inMemorySupplier struct {
	entries []string
	pos     int
}

func (s *inMemorySupplier) WordCount() int { return len(s.entries) }

func (s *inMemorySupplier) Next() (string, error) {
	if s.pos >= len(s.entries) {
		return "", nil
	}
	v := s.entries[s.pos]
	s.pos++
	return v, nil
}

func (s *inMemorySupplier) Close() error { return nil }

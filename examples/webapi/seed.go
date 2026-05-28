// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package webapi

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed books.json
var seedCorpus []byte

// GoldenCorpus decodes and returns the embedded seed corpus. The slice is
// freshly allocated on each call so callers can mutate it without affecting
// other readers.
func GoldenCorpus() ([]Book, error) {
	var books []Book
	if err := json.Unmarshal(seedCorpus, &books); err != nil {
		return nil, fmt.Errorf("decode embedded corpus: %w", err)
	}
	return books, nil
}

// SeedIfEmpty populates the store with the golden corpus when (and only when)
// the underlying index is currently empty. The number of inserted books is
// returned for logging purposes.
//
// All books are added to the shadow map first and then indexed in a single
// bulk rebuild. This avoids the repeated DeleteAll→AddDocument→Commit cycles
// that a per-book Put would trigger, which can leave the directory in a state
// where field-level term queries produce incorrect results.
func SeedIfEmpty(s *BookStore) (int, error) {
	empty, err := s.IsEmpty()
	if err != nil {
		return 0, err
	}
	if !empty {
		return 0, nil
	}

	books, err := GoldenCorpus()
	if err != nil {
		return 0, err
	}
	if len(books) == 0 {
		return 0, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range books {
		if books[i].ID == "" {
			id, err := generateID()
			if err != nil {
				return i, fmt.Errorf("generate id for book %d: %w", i, err)
			}
			books[i].ID = id
		}
		if _, existed := s.books[books[i].ID]; !existed {
			s.order = append(s.order, books[i].ID)
		}
		s.books[books[i].ID] = books[i]
	}

	if err := s.rebuildIndexLocked(); err != nil {
		return 0, fmt.Errorf("bulk seed rebuild: %w", err)
	}
	return len(books), nil
}

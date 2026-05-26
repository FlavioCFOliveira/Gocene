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
	for i := range books {
		if err := s.Put(&books[i]); err != nil {
			return i, fmt.Errorf("seed book %q: %w", books[i].ID, err)
		}
	}
	return len(books), nil
}

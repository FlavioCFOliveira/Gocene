// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

// Root represents a dictionary root word paired with its entry id.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.Root from Apache Lucene 10.4.0.
//
// Deviation: Java uses a generic record Root<T extends CharSequence>; Go uses
// a struct with a plain string, which covers all call sites in Gocene.
type Root struct {
	Word    string
	EntryID int
}

// NewRoot constructs a Root.
func NewRoot(word string, entryID int) *Root {
	return &Root{Word: word, EntryID: entryID}
}

func (r *Root) String() string { return r.Word }

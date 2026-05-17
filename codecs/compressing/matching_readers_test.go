// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Tests for MatchingReaders. No direct TestMatchingReaders.java peer
// exists in Lucene 10.4.0; the cases below exercise the documented
// branches of the constructor against the minimal MergeState placeholder
// declared in matching_readers.go.

package compressing

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// makeFieldInfos returns a *FieldInfos populated with name<->number
// entries from the given (name, number) tuples in declaration order.
func makeFieldInfos(t *testing.T, entries ...struct {
	name   string
	number int
}) *index.FieldInfos {
	t.Helper()
	fis := index.NewFieldInfos()
	for _, e := range entries {
		opts := index.DefaultFieldInfoOptions()
		fi := index.NewFieldInfo(e.name, e.number, opts)
		if err := fis.Add(fi); err != nil {
			t.Fatalf("FieldInfos.Add(%q, %d): %v", e.name, e.number, err)
		}
	}
	return fis
}

func TestMatchingReaders_NilMergeState(t *testing.T) {
	mr := NewMatchingReaders(nil)
	if mr == nil {
		t.Fatal("NewMatchingReaders(nil) returned nil")
	}
	if mr.Count != 0 {
		t.Errorf("Count = %d, want 0", mr.Count)
	}
	if mr.MatchingReaders != nil && len(mr.MatchingReaders) != 0 {
		t.Errorf("MatchingReaders = %v, want empty", mr.MatchingReaders)
	}
}

func TestMatchingReaders_NoReaders(t *testing.T) {
	mr := NewMatchingReaders(&MergeState{
		MaxDocs:         nil,
		FieldInfos:      nil,
		MergeFieldInfos: index.NewFieldInfos(),
	})
	if mr.Count != 0 {
		t.Errorf("Count = %d, want 0", mr.Count)
	}
	if len(mr.MatchingReaders) != 0 {
		t.Errorf("len(MatchingReaders) = %d, want 0", len(mr.MatchingReaders))
	}
}

func TestMatchingReaders_AllMatch(t *testing.T) {
	type fe = struct {
		name   string
		number int
	}
	merged := makeFieldInfos(t, fe{"title", 0}, fe{"body", 1}, fe{"id", 2})
	reader0 := makeFieldInfos(t, fe{"title", 0}, fe{"body", 1})
	reader1 := makeFieldInfos(t, fe{"id", 2})

	mr := NewMatchingReaders(&MergeState{
		MaxDocs:         []int{10, 20},
		FieldInfos:      []*index.FieldInfos{reader0, reader1},
		MergeFieldInfos: merged,
	})
	if mr.Count != 2 {
		t.Errorf("Count = %d, want 2", mr.Count)
	}
	if got := mr.MatchingReaders; len(got) != 2 || !got[0] || !got[1] {
		t.Errorf("MatchingReaders = %v, want [true true]", got)
	}
}

func TestMatchingReaders_NameMismatch(t *testing.T) {
	type fe = struct {
		name   string
		number int
	}
	// Field number 1 in the merged FieldInfos is "body", but reader1 has
	// it as "content" — same number, different name → non-bulk merge.
	merged := makeFieldInfos(t, fe{"title", 0}, fe{"body", 1})
	reader0 := makeFieldInfos(t, fe{"title", 0}, fe{"body", 1})
	reader1 := makeFieldInfos(t, fe{"title", 0}, fe{"content", 1})

	mr := NewMatchingReaders(&MergeState{
		MaxDocs:         []int{10, 20},
		FieldInfos:      []*index.FieldInfos{reader0, reader1},
		MergeFieldInfos: merged,
	})
	if mr.Count != 1 {
		t.Errorf("Count = %d, want 1", mr.Count)
	}
	want := []bool{true, false}
	for i := range want {
		if mr.MatchingReaders[i] != want[i] {
			t.Errorf("MatchingReaders[%d] = %v, want %v", i, mr.MatchingReaders[i], want[i])
		}
	}
}

func TestMatchingReaders_UnknownNumber(t *testing.T) {
	type fe = struct {
		name   string
		number int
	}
	// reader1 references field number 5, which doesn't exist in merged
	// → other == nil branch in the Java reference.
	merged := makeFieldInfos(t, fe{"title", 0}, fe{"body", 1})
	reader0 := makeFieldInfos(t, fe{"title", 0})
	reader1 := makeFieldInfos(t, fe{"body", 1}, fe{"orphan", 5})

	mr := NewMatchingReaders(&MergeState{
		MaxDocs:         []int{10, 20},
		FieldInfos:      []*index.FieldInfos{reader0, reader1},
		MergeFieldInfos: merged,
	})
	if mr.Count != 1 {
		t.Errorf("Count = %d, want 1", mr.Count)
	}
	if !mr.MatchingReaders[0] || mr.MatchingReaders[1] {
		t.Errorf("MatchingReaders = %v, want [true false]", mr.MatchingReaders)
	}
}

func TestMatchingReaders_EmptyReaderFieldInfos(t *testing.T) {
	type fe = struct {
		name   string
		number int
	}
	// A reader with zero fields trivially matches: the inner for-loop
	// never enters the mismatch branch.
	merged := makeFieldInfos(t, fe{"title", 0})
	emptyReader := index.NewFieldInfos()
	nonEmpty := makeFieldInfos(t, fe{"title", 0})

	mr := NewMatchingReaders(&MergeState{
		MaxDocs:         []int{1, 1},
		FieldInfos:      []*index.FieldInfos{emptyReader, nonEmpty},
		MergeFieldInfos: merged,
	})
	if mr.Count != 2 {
		t.Errorf("Count = %d, want 2", mr.Count)
	}
}

func TestMatchingReaders_NilMergedFieldInfos(t *testing.T) {
	// Defensive branch: nil MergeFieldInfos is treated as "no field
	// matches at all"; no reader can be bulk-merged.
	reader0 := makeFieldInfos(t, struct {
		name   string
		number int
	}{"title", 0})

	mr := NewMatchingReaders(&MergeState{
		MaxDocs:         []int{1},
		FieldInfos:      []*index.FieldInfos{reader0},
		MergeFieldInfos: nil,
	})
	if mr.Count != 0 {
		t.Errorf("Count = %d, want 0", mr.Count)
	}
	if mr.MatchingReaders[0] {
		t.Errorf("MatchingReaders[0] = true, want false")
	}
}

func TestMatchingReaders_UnderSizedFieldInfosSlice(t *testing.T) {
	// Defensive branch: MaxDocs declares 2 readers but FieldInfos has 1.
	// The missing entry must be treated as a non-match (zero panic).
	type fe = struct {
		name   string
		number int
	}
	merged := makeFieldInfos(t, fe{"title", 0})
	reader0 := makeFieldInfos(t, fe{"title", 0})

	mr := NewMatchingReaders(&MergeState{
		MaxDocs:         []int{10, 20},
		FieldInfos:      []*index.FieldInfos{reader0}, // length 1, not 2
		MergeFieldInfos: merged,
	})
	if mr.Count != 1 {
		t.Errorf("Count = %d, want 1", mr.Count)
	}
	if !mr.MatchingReaders[0] || mr.MatchingReaders[1] {
		t.Errorf("MatchingReaders = %v, want [true false]", mr.MatchingReaders)
	}
}

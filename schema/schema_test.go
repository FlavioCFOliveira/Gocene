// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package schema

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// indexedOpts returns FieldInfoOptions describing an indexed field with the
// given index options, starting from the canonical defaults.
func indexedOpts(io IndexOptions) FieldInfoOptions {
	opts := DefaultFieldInfoOptions()
	opts.IndexOptions = io
	return opts
}

// TestFieldInfoGetters verifies the constructor copies options through to the
// accessor surface unchanged for a straightforward indexed field.
func TestFieldInfoGetters(t *testing.T) {
	t.Parallel()
	opts := indexedOpts(IndexOptionsDocsAndFreqsAndPositions)
	opts.DocValuesType = DocValuesTypeSorted
	opts.Stored = true
	fi := NewFieldInfo("title", 3, opts)

	if got := fi.Name(); got != "title" {
		t.Errorf("Name() = %q, want title", got)
	}
	if got := fi.Number(); got != 3 {
		t.Errorf("Number() = %d, want 3", got)
	}
	if got := fi.IndexOptions(); got != IndexOptionsDocsAndFreqsAndPositions {
		t.Errorf("IndexOptions() = %v, want DocsAndFreqsAndPositions", got)
	}
	if got := fi.DocValuesType(); got != DocValuesTypeSorted {
		t.Errorf("DocValuesType() = %v, want Sorted", got)
	}
	if !fi.IsStored() {
		t.Error("IsStored() = false, want true")
	}
	if !fi.IsFrozen() {
		t.Error("IsFrozen() = false after construction, want true")
	}
}

// TestFieldInfoTermVectorNormalization pins the constructor's auto-correction:
// requesting a term-vector component (positions/offsets/payloads) without the
// base storeTermVectors flag promotes the base flag to true, matching Lucene's
// FieldInfo constructor invariants.
func TestFieldInfoTermVectorNormalization(t *testing.T) {
	t.Parallel()
	opts := indexedOpts(IndexOptionsDocsAndFreqsAndPositions)
	opts.StoreTermVectorPositions = true // implies storeTermVectors
	fi := NewFieldInfo("body", 0, opts)

	if !fi.StoreTermVectors() {
		t.Error("StoreTermVectors() = false when positions requested, want true (auto-promoted)")
	}
	if !fi.StoreTermVectorPositions() {
		t.Error("StoreTermVectorPositions() = false, want true")
	}
	if !fi.HasTermVectors() {
		t.Error("HasTermVectors() = false, want true")
	}
}

// TestFieldInfoTokenizedRequiresIndexing pins the other constructor invariant:
// a tokenized-but-not-indexed field has its tokenized flag cleared.
func TestFieldInfoTokenizedRequiresIndexing(t *testing.T) {
	t.Parallel()
	opts := DefaultFieldInfoOptions() // IndexOptionsNone
	opts.Tokenized = true
	fi := NewFieldInfo("meta", 0, opts)

	if fi.IsTokenized() {
		t.Error("IsTokenized() = true on a non-indexed field, want false (auto-cleared)")
	}
}

// TestFieldInfoHasNorms checks the derived HasNorms predicate across index
// options and the omitNorms flag.
func TestFieldInfoHasNorms(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		io        IndexOptions
		omitNorms bool
		want      bool
	}{
		{"not indexed", IndexOptionsNone, false, false},
		{"docs only (no freqs)", IndexOptionsDocs, false, false},
		{"docs+freqs", IndexOptionsDocsAndFreqs, false, true},
		{"docs+freqs, omitNorms", IndexOptionsDocsAndFreqs, true, false},
		{"full", IndexOptionsDocsAndFreqsAndPositionsAndOffsets, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := indexedOpts(tc.io)
			opts.OmitNorms = tc.omitNorms
			fi := NewFieldInfo("f", 0, opts)
			if got := fi.HasNorms(); got != tc.want {
				t.Errorf("HasNorms() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestFieldInfoStorePayloads verifies SetStorePayloads honours the
// positions-required guard and flips HasPayloads only for eligible fields.
func TestFieldInfoStorePayloads(t *testing.T) {
	t.Parallel()

	// Field without positions: SetStorePayloads is a no-op.
	low := NewFieldInfo("f1", 0, indexedOpts(IndexOptionsDocsAndFreqs))
	if low.HasPayloads() {
		t.Error("fresh field HasPayloads() = true, want false")
	}
	low.SetStorePayloads()
	if low.HasPayloads() {
		t.Error("HasPayloads() = true after SetStorePayloads on a no-positions field, want false (guarded)")
	}

	// Field with positions: SetStorePayloads takes effect.
	withPos := NewFieldInfo("f2", 1, indexedOpts(IndexOptionsDocsAndFreqsAndPositions))
	withPos.SetStorePayloads()
	if !withPos.HasPayloads() {
		t.Error("HasPayloads() = false after SetStorePayloads on a positions field, want true")
	}
	if !withPos.HasStoredPayloads() {
		t.Error("HasStoredPayloads() = false after SetStorePayloads, want true")
	}
}

// TestFieldInfoAttributes covers attribute reads, the frozen-write panic on
// PutAttribute, and the codec escape hatch PutCodecAttribute.
func TestFieldInfoAttributes(t *testing.T) {
	t.Parallel()
	fi := NewFieldInfo("f", 0, indexedOpts(IndexOptionsDocs))

	if got := fi.GetAttribute("missing"); got != "" {
		t.Errorf("GetAttribute(missing) = %q, want empty", got)
	}

	// PutAttribute on a frozen FieldInfo must panic.
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("PutAttribute on frozen FieldInfo did not panic")
			}
		}()
		fi.PutAttribute("k", "v")
	}()

	// PutCodecAttribute bypasses the frozen contract.
	fi.PutCodecAttribute("PerFieldPostingsFormat.format", "Lucene103")
	if got := fi.GetAttribute("PerFieldPostingsFormat.format"); got != "Lucene103" {
		t.Errorf("GetAttribute after PutCodecAttribute = %q, want Lucene103", got)
	}

	// GetAttributes returns a defensive copy.
	attrs := fi.GetAttributes()
	attrs["PerFieldPostingsFormat.format"] = "tampered"
	if fi.GetAttribute("PerFieldPostingsFormat.format") != "Lucene103" {
		t.Error("mutating GetAttributes() result leaked into FieldInfo")
	}
}

// TestFieldInfoClone verifies Clone copies attributes and re-freezes with a new
// field number, independent of the source.
func TestFieldInfoClone(t *testing.T) {
	t.Parallel()
	src := NewFieldInfo("orig", 2, indexedOpts(IndexOptionsDocsAndFreqsAndPositions))
	src.PutCodecAttribute("a", "1")

	clone := src.Clone(9)
	if clone.Name() != "orig" {
		t.Errorf("clone Name() = %q, want orig", clone.Name())
	}
	if clone.Number() != 9 {
		t.Errorf("clone Number() = %d, want 9", clone.Number())
	}
	if clone.GetAttribute("a") != "1" {
		t.Errorf("clone attribute a = %q, want 1", clone.GetAttribute("a"))
	}
	if !clone.IsFrozen() {
		t.Error("clone IsFrozen() = false, want true")
	}

	// Mutating the clone's codec attributes must not touch the source.
	clone.PutCodecAttribute("a", "changed")
	if src.GetAttribute("a") != "1" {
		t.Errorf("source attribute changed to %q after clone mutation, want 1", src.GetAttribute("a"))
	}
}

// TestFieldInfoCheckConsistency verifies the validator rejects an inconsistent
// state and accepts a valid one. The inconsistent state is built with the
// Override* test escape hatches, since the constructor would auto-correct it.
func TestFieldInfoCheckConsistency(t *testing.T) {
	t.Parallel()

	valid := NewFieldInfo("ok", 0, indexedOpts(IndexOptionsDocsAndFreqsAndPositions))
	if err := valid.CheckConsistency(); err != nil {
		t.Errorf("CheckConsistency() on a valid FieldInfo returned %v, want nil", err)
	}

	// Force an inconsistent state: positions stored without base term vectors.
	bad := NewFieldInfo("bad", 1, func() FieldInfoOptions {
		o := indexedOpts(IndexOptionsDocsAndFreqsAndPositions)
		o.StoreTermVectorPositions = true
		return o
	}())
	bad.OverrideStoreTermVectors(false) // break the invariant the ctor enforced
	if err := bad.CheckConsistency(); err == nil {
		t.Error("CheckConsistency() accepted positions-without-vectors, want error")
	}
}

// TestFieldInfosAddAndLookup covers Add, name/number lookup, Size, and Names
// ordering.
func TestFieldInfosAddAndLookup(t *testing.T) {
	t.Parallel()
	infos := NewFieldInfos()

	a := NewFieldInfo("alpha", 0, indexedOpts(IndexOptionsDocs))
	b := NewFieldInfo("beta", 1, indexedOpts(IndexOptionsDocsAndFreqs))
	if err := infos.Add(a); err != nil {
		t.Fatalf("Add(alpha): %v", err)
	}
	if err := infos.Add(b); err != nil {
		t.Fatalf("Add(beta): %v", err)
	}

	if got := infos.Size(); got != 2 {
		t.Fatalf("Size() = %d, want 2", got)
	}
	if got := infos.GetByName("alpha"); got != a {
		t.Errorf("GetByName(alpha) = %v, want a", got)
	}
	if got := infos.GetByNumber(1); got != b {
		t.Errorf("GetByNumber(1) = %v, want b", got)
	}
	if got := infos.GetByName("missing"); got != nil {
		t.Errorf("GetByName(missing) = %v, want nil", got)
	}

	// Names are returned sorted.
	names := infos.Names()
	if len(names) != 2 || names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("Names() = %v, want [alpha beta]", names)
	}

	// GetNextFieldNumber advanced past the highest used number.
	if got := infos.GetNextFieldNumber(); got != 2 {
		t.Errorf("GetNextFieldNumber() = %d, want 2", got)
	}
}

// TestFieldInfosAddConflicts verifies the duplicate-handling rules: re-adding
// the identical field is a no-op, while a name/number conflict is an error.
func TestFieldInfosAddConflicts(t *testing.T) {
	t.Parallel()
	infos := NewFieldInfos()
	a := NewFieldInfo("alpha", 0, indexedOpts(IndexOptionsDocs))
	if err := infos.Add(a); err != nil {
		t.Fatalf("Add(alpha): %v", err)
	}

	// Same name + same number: ignored, no error, size unchanged.
	if err := infos.Add(a); err != nil {
		t.Errorf("re-adding identical FieldInfo returned %v, want nil", err)
	}
	if got := infos.Size(); got != 1 {
		t.Errorf("Size() after duplicate add = %d, want 1", got)
	}

	// Same name, different number: error.
	dupName := NewFieldInfo("alpha", 5, indexedOpts(IndexOptionsDocs))
	if err := infos.Add(dupName); err == nil {
		t.Error("Add(alpha#5) accepted a name with a different number, want error")
	}

	// Same number, different name: error.
	dupNum := NewFieldInfo("gamma", 0, indexedOpts(IndexOptionsDocs))
	if err := infos.Add(dupNum); err == nil {
		t.Error("Add(gamma#0) accepted a re-used number, want error")
	}
}

// TestFieldInfosAggregates checks the Has* aggregate predicates across a small
// field set.
func TestFieldInfosAggregates(t *testing.T) {
	t.Parallel()
	infos := NewFieldInfos()

	// One field with positions+offsets+payloads, one stored doc-values field.
	withProx := func() FieldInfoOptions {
		o := indexedOpts(IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
		return o
	}()
	if err := infos.Add(NewFieldInfo("text", 0, withProx)); err != nil {
		t.Fatalf("Add(text): %v", err)
	}
	dvOpts := DefaultFieldInfoOptions()
	dvOpts.DocValuesType = DocValuesTypeNumeric
	if err := infos.Add(NewFieldInfo("count", 1, dvOpts)); err != nil {
		t.Fatalf("Add(count): %v", err)
	}

	if !infos.HasProx() {
		t.Error("HasProx() = false, want true (text has positions)")
	}
	if !infos.HasFreq() {
		t.Error("HasFreq() = false, want true")
	}
	if !infos.HasOffsets() {
		t.Error("HasOffsets() = false, want true")
	}
	if !infos.HasDocValues() {
		t.Error("HasDocValues() = false, want true (count is numeric DV)")
	}
	if !infos.HasPostings() {
		t.Error("HasPostings() = false, want true")
	}
}

// TestFieldInfosFreeze verifies Freeze makes the collection immutable.
func TestFieldInfosFreeze(t *testing.T) {
	t.Parallel()
	infos := NewFieldInfos()
	if infos.IsFrozen() {
		t.Error("fresh FieldInfos IsFrozen() = true, want false")
	}
	infos.Freeze()
	if !infos.IsFrozen() {
		t.Error("IsFrozen() = false after Freeze, want true")
	}
	if err := infos.Add(NewFieldInfo("x", 0, indexedOpts(IndexOptionsDocs))); err == nil {
		t.Error("Add on a frozen FieldInfos succeeded, want error")
	}
}

// TestFieldInfosBuilder exercises the fluent builder path.
func TestFieldInfosBuilder(t *testing.T) {
	t.Parallel()
	b := NewFieldInfosBuilder().
		Add(NewFieldInfo("a", 0, indexedOpts(IndexOptionsDocs))).
		AddFromOptions("b", indexedOpts(IndexOptionsDocsAndFreqs))
	infos := b.Build()

	if got := infos.Size(); got != 2 {
		t.Fatalf("builder produced Size() = %d, want 2", got)
	}
	if infos.GetByName("a") == nil || infos.GetByName("b") == nil {
		t.Error("builder did not register both fields")
	}
}

// TestFieldInfoBuilder exercises the single-FieldInfo fluent builder and its
// normalization (which mirrors NewFieldInfo).
func TestFieldInfoBuilder(t *testing.T) {
	t.Parallel()
	fi := NewFieldInfoBuilder("v", 4).
		SetIndexOptions(IndexOptionsDocsAndFreqsAndPositions).
		SetStoreTermVectorOffsets(true). // implies storeTermVectors
		SetStored(true).
		Build()

	if fi.Name() != "v" || fi.Number() != 4 {
		t.Errorf("builder identity = (%q,%d), want (v,4)", fi.Name(), fi.Number())
	}
	if !fi.StoreTermVectors() {
		t.Error("builder did not auto-promote storeTermVectors for offsets")
	}
	if !fi.IsStored() {
		t.Error("builder SetStored(true) not reflected in IsStored()")
	}
}

// TestSegmentInfoBasics covers construction, file accounting, version, codec,
// and the 16-byte id invariant.
func TestSegmentInfoBasics(t *testing.T) {
	t.Parallel()
	dir := store.NewByteBuffersDirectory()
	si := NewSegmentInfo("_5", 42, dir)

	if si.Name() != "_5" {
		t.Errorf("Name() = %q, want _5", si.Name())
	}
	if si.DocCount() != 42 {
		t.Errorf("DocCount() = %d, want 42", si.DocCount())
	}
	if si.Directory() != dir {
		t.Error("Directory() did not return the constructor argument")
	}

	// Files: SetFiles sorts; AddFile keeps sorted; HasFile reports membership.
	si.SetFiles([]string{"_5.fdt", "_5.fdx"})
	si.AddFile("_5.cfe")
	files := si.Files()
	if len(files) != 3 || files[0] != "_5.cfe" {
		t.Errorf("Files() = %v, want sorted with _5.cfe first", files)
	}
	if !si.HasFile("_5.fdt") {
		t.Error("HasFile(_5.fdt) = false, want true")
	}
	// Files() returns a copy.
	files[0] = "tampered"
	if si.HasFile("tampered") {
		t.Error("mutating Files() result leaked into SegmentInfo")
	}

	si.SetCodec("Lucene103")
	if si.Codec() != "Lucene103" {
		t.Errorf("Codec() = %q, want Lucene103", si.Codec())
	}

	// SetID requires exactly 16 bytes.
	if err := si.SetID([]byte{1, 2, 3}); err == nil {
		t.Error("SetID with 3 bytes succeeded, want error")
	}
	id := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	if err := si.SetID(id); err != nil {
		t.Fatalf("SetID with 16 bytes: %v", err)
	}
	got := si.GetID()
	if len(got) != 16 || got[15] != 15 {
		t.Errorf("GetID() = % d, want the 16-byte id set above", got)
	}
	// GetID returns a copy.
	got[0] = 0xFF
	if si.GetID()[0] == 0xFF {
		t.Error("mutating GetID() result leaked into SegmentInfo")
	}
}

// TestSegmentInfoIndexSort covers the index-sort accessor and its
// human-readable description.
func TestSegmentInfoIndexSort(t *testing.T) {
	t.Parallel()
	si := NewSegmentInfo("_6", 1, store.NewByteBuffersDirectory())

	if got := si.GetIndexSortDescription(); got != "<not sorted>" {
		t.Errorf("unsorted description = %q, want <not sorted>", got)
	}

	sort := NewSort(
		NewSortFieldFull("price", 0, true), // descending
		NewSortFieldFull("name", 0, false), // ascending
	)
	si.SetIndexSort(sort)
	if si.IndexSort() != sort {
		t.Error("IndexSort() did not return the sort set")
	}
	if got := si.GetIndexSortDescription(); got != "price DESC, name ASC" {
		t.Errorf("sort description = %q, want %q", got, "price DESC, name ASC")
	}
}

// TestSegmentInfoMinVersionAndHasBlocks covers the rmp #4784 accessors:
// MinVersion()/SetMinVersion default to absent and round-trip a value (with the
// empty string clearing it), and HasBlocks()/SetHasBlocks default to false.
func TestSegmentInfoMinVersionAndHasBlocks(t *testing.T) {
	t.Parallel()
	si := NewSegmentInfo("_7", 1, store.NewByteBuffersDirectory())

	// MinVersion defaults to absent (nil), matching Lucene's SegmentInfo where
	// getMinVersion() returns null when no version was explicitly set. The .si
	// writer emits the hasMinVersion=0 byte in this case.
	if v, ok := si.MinVersion(); ok || v != "" {
		t.Errorf("default MinVersion = (%q, %v), want (\"\", false)", v, ok)
	}
	if si.HasBlocks() {
		t.Error("default HasBlocks = true, want false")
	}

	si.SetMinVersion("9.11.1")
	if v, ok := si.MinVersion(); !ok || v != "9.11.1" {
		t.Errorf("MinVersion after set = (%q, %v), want (\"9.11.1\", true)", v, ok)
	}
	si.SetMinVersion("")
	if v, ok := si.MinVersion(); ok || v != "" {
		t.Errorf("MinVersion after clear = (%q, %v), want (\"\", false)", v, ok)
	}

	si.SetHasBlocks(true)
	if !si.HasBlocks() {
		t.Error("HasBlocks after SetHasBlocks(true) = false, want true")
	}

	// Clone must carry both fields.
	si.SetMinVersion("10.0.0")
	clone := si.Clone()
	if v, ok := clone.MinVersion(); !ok || v != "10.0.0" {
		t.Errorf("clone MinVersion = (%q, %v), want (\"10.0.0\", true)", v, ok)
	}
	if !clone.HasBlocks() {
		t.Error("clone HasBlocks = false, want true")
	}
}

// TestSortField checks the SortField value type's accessors and reverse flag.
func TestSortField(t *testing.T) {
	t.Parallel()
	sf := NewSortField("field", 0)
	if sf.Field() != "field" {
		t.Errorf("Field() = %q, want field", sf.Field())
	}
	if sf.Descending() {
		t.Error("Descending() = true for a forward SortField, want false")
	}
	sf.SetReverse(true)
	if !sf.Descending() {
		t.Error("Descending() = false after SetReverse(true), want true")
	}

	full := NewSortFieldFull("f2", 0, true)
	if !full.Descending() {
		t.Error("NewSortFieldFull(descending=true).Descending() = false, want true")
	}

	sort := NewSort(sf, full)
	if got := sort.Fields(); len(got) != 2 {
		t.Errorf("Sort.Fields() len = %d, want 2", len(got))
	}
}

// TestFieldInfoVerifyAndUpdate covers the per-field schema accumulation that
// backs the dual-purpose-field fix (rmp #4780). A FieldInfo created from one
// contribution must accumulate the complementary group from a later
// contribution; a NONE / zero value must never clear an already-set group; a
// conflicting non-default value must be reported as an error.
func TestFieldInfoVerifyAndUpdate(t *testing.T) {
	t.Parallel()

	t.Run("indexed then sorted DV accumulates both", func(t *testing.T) {
		fi := NewFieldInfo("f", 0, indexedOpts(IndexOptionsDocs))

		dvOpts := DefaultFieldInfoOptions()
		dvOpts.DocValuesType = DocValuesTypeSorted
		if err := fi.VerifyAndUpdate(dvOpts); err != nil {
			t.Fatalf("VerifyAndUpdate(sorted DV): %v", err)
		}
		if fi.IndexOptions() != IndexOptionsDocs {
			t.Errorf("IndexOptions = %s, want DOCS (indexed contribution must survive)", fi.IndexOptions())
		}
		if fi.DocValuesType() != DocValuesTypeSorted {
			t.Errorf("DocValuesType = %s, want SORTED (DV contribution must be adopted)", fi.DocValuesType())
		}
	})

	t.Run("sorted DV then indexed accumulates both", func(t *testing.T) {
		dvOpts := DefaultFieldInfoOptions()
		dvOpts.DocValuesType = DocValuesTypeSorted
		fi := NewFieldInfo("f", 0, dvOpts)

		if err := fi.VerifyAndUpdate(indexedOpts(IndexOptionsDocs)); err != nil {
			t.Fatalf("VerifyAndUpdate(indexed): %v", err)
		}
		if fi.IndexOptions() != IndexOptionsDocs {
			t.Errorf("IndexOptions = %s, want DOCS", fi.IndexOptions())
		}
		if fi.DocValuesType() != DocValuesTypeSorted {
			t.Errorf("DocValuesType = %s, want SORTED (DV contribution must survive)", fi.DocValuesType())
		}
	})

	t.Run("NONE never clears a set group", func(t *testing.T) {
		opts := indexedOpts(IndexOptionsDocs)
		opts.DocValuesType = DocValuesTypeNumeric
		fi := NewFieldInfo("f", 0, opts)

		// A purely-default (NONE) contribution must not downgrade either group.
		if err := fi.VerifyAndUpdate(DefaultFieldInfoOptions()); err != nil {
			t.Fatalf("VerifyAndUpdate(defaults): %v", err)
		}
		if fi.IndexOptions() != IndexOptionsDocs {
			t.Errorf("IndexOptions cleared to %s by a NONE contribution", fi.IndexOptions())
		}
		if fi.DocValuesType() != DocValuesTypeNumeric {
			t.Errorf("DocValuesType cleared to %s by a NONE contribution", fi.DocValuesType())
		}
	})

	t.Run("conflicting doc values type is an error", func(t *testing.T) {
		dvOpts := DefaultFieldInfoOptions()
		dvOpts.DocValuesType = DocValuesTypeSorted
		fi := NewFieldInfo("f", 0, dvOpts)

		conflicting := DefaultFieldInfoOptions()
		conflicting.DocValuesType = DocValuesTypeNumeric
		if err := fi.VerifyAndUpdate(conflicting); err == nil {
			t.Error("VerifyAndUpdate with conflicting DV type returned nil, want error")
		}
		if fi.DocValuesType() != DocValuesTypeSorted {
			t.Errorf("DocValuesType mutated to %s on conflict, want SORTED preserved", fi.DocValuesType())
		}
	})

	t.Run("conflicting index options is an error", func(t *testing.T) {
		fi := NewFieldInfo("f", 0, indexedOpts(IndexOptionsDocs))
		if err := fi.VerifyAndUpdate(indexedOpts(IndexOptionsDocsAndFreqsAndPositions)); err == nil {
			t.Error("VerifyAndUpdate with conflicting index options returned nil, want error")
		}
		if fi.IndexOptions() != IndexOptionsDocs {
			t.Errorf("IndexOptions mutated to %s on conflict, want DOCS preserved", fi.IndexOptions())
		}
	})

	t.Run("point and vector dims accumulate", func(t *testing.T) {
		// Start from a point field, accumulate a vector contribution.
		ptOpts := DefaultFieldInfoOptions()
		ptOpts.PointDimensionCount = 1
		ptOpts.PointIndexDimensionCount = 1
		ptOpts.PointNumBytes = 4
		fi := NewFieldInfo("f", 0, ptOpts)

		vecOpts := DefaultFieldInfoOptions()
		vecOpts.VectorDimension = 3
		vecOpts.VectorEncoding = VectorEncodingFloat32
		vecOpts.VectorSimilarityFunction = VectorSimilarityFunctionEuclidean
		if err := fi.VerifyAndUpdate(vecOpts); err != nil {
			t.Fatalf("VerifyAndUpdate(vector): %v", err)
		}
		if fi.PointDimensionCount() != 1 || fi.PointNumBytes() != 4 {
			t.Errorf("point dims lost: count=%d bytes=%d", fi.PointDimensionCount(), fi.PointNumBytes())
		}
		if fi.VectorDimension() != 3 {
			t.Errorf("VectorDimension = %d, want 3", fi.VectorDimension())
		}
	})
}

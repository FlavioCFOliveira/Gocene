// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestSegmentInfos.java
// (Apache Lucene 10.5.0).

package index

import (
	"fmt"
	"maps"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// segmentInfosTestSegmentInfo renders the twelve-argument constructor
// new SegmentInfo(dir, version, minVersion, name, maxDoc, isCompoundFile,
// hasBlocks, codec, diagnostics, id, attributes, indexSort), which Gocene
// spells as NewSegmentInfo followed by the setters.
func segmentInfosTestSegmentInfo(t *testing.T, dir store.Directory, version, minVersion, name string, maxDoc int,
	codec Codec, diagnostics map[string]string, id []byte, attributes map[string]string, indexSort *Sort) *SegmentInfo {
	t.Helper()
	si := NewSegmentInfo(name, maxDoc, dir)
	si.SetVersion(version)
	si.SetMinVersion(minVersion)
	si.SetUseCompoundFile(false)
	si.SetHasBlocks(false)
	si.SetCodec(codec)
	si.SetDiagnostics(maps.Clone(diagnostics))
	if err := si.SetID(id); err != nil {
		t.Fatalf("setID: %v", err)
	}
	si.SetAttributes(attributes)
	if indexSort != nil {
		si.SetIndexSort(indexSort)
	}
	return si
}

// segmentInfosIndexOrder renders Sort.INDEXORDER: new Sort(SortField.FIELD_DOC).
func segmentInfosIndexOrder() *Sort {
	return NewSortFromFields([]SortField{*NewSortField("", SortTypeDoc)})
}

func TestSegmentInfosIllegalCreatedVersion(t *testing.T) {
	expectIllegalArgument := func(major int32, want string) {
		t.Helper()
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("expected IllegalArgumentException for %d", major)
			}
			if got := fmt.Sprint(r); got != want {
				t.Fatalf("expected %q, got %q", want, got)
			}
		}()
		spi.NewSegmentInfosWithCreatedVersionMajor(major)
	}
	expectIllegalArgument(5, "indexCreatedVersionMajor must be >= 6, got: 5")
	expectIllegalArgument(int32(util.Latest.Major+1),
		fmt.Sprintf("indexCreatedVersionMajor is in the future: %d", util.Latest.Major+1))
}

const segmentInfosCommitMissing = "org.apache.lucene.index.SegmentInfos#commit(Directory), " +
	"#getMinSegmentLuceneVersion() and #getCommitLuceneVersion(), " +
	"org.apache.lucene.util.Version#LUCENE_10_0_0 and " +
	"org.apache.lucene.tests.store.BaseDirectoryWrapper#setCheckIndexOnClose(boolean) are not ported"

// TestSegmentInfosVersionsNoSegments ports testVersionsNoSegments (LUCENE-5954).
func TestSegmentInfosVersionsNoSegments(t *testing.T) {
	t.Fatal(segmentInfosCommitMissing)
}

// TestSegmentInfosVersionsOneSegment ports testVersionsOneSegment (LUCENE-5954).
func TestSegmentInfosVersionsOneSegment(t *testing.T) {
	t.Fatal(segmentInfosCommitMissing)
}

// TestSegmentInfosVersionsTwoSegments ports testVersionsTwoSegments (LUCENE-5954).
func TestSegmentInfosVersionsTwoSegments(t *testing.T) {
	t.Fatal(segmentInfosCommitMissing)
}

// TestSegmentInfosToString ports testToString, which asserts
// SegmentInfo#toStringVerbose() next to SegmentInfo#toString().
func TestSegmentInfosToString(t *testing.T) {
	t.Fatal("org.apache.lucene.index.SegmentInfo#toStringVerbose() is not ported")
}

func TestSegmentInfosIDChangesOnAdvance(t *testing.T) {
	dir := newDirectory()
	defer dir.Close()
	id := util.RandomId()
	info := segmentInfosTestSegmentInfo(t, dir, "9.0.0", "9.0.0", "_0", 1, GetDefaultCodec(),
		map[string]string{}, util.RandomId(), map[string]string{}, nil)
	commitInfo := NewSegmentCommitInfo(info, 0, 0, -1, -1, -1, id)
	if util.IdToString(id) != util.IdToString(commitInfo.GetID()) {
		t.Fatal("assertEquals(id, commitInfo.getId())")
	}
	commitInfo.AdvanceDelGen()
	if util.IdToString(id) == util.IdToString(commitInfo.GetID()) {
		t.Fatal("assertNotEquals after advanceDelGen")
	}

	id = commitInfo.GetID()
	commitInfo.AdvanceDocValuesGen()
	if util.IdToString(id) == util.IdToString(commitInfo.GetID()) {
		t.Fatal("assertNotEquals after advanceDocValuesGen")
	}

	id = commitInfo.GetID()
	commitInfo.AdvanceFieldInfosGen()
	if util.IdToString(id) == util.IdToString(commitInfo.GetID()) {
		t.Fatal("assertNotEquals after advanceFieldInfosGen")
	}
	clone := commitInfo.Clone()
	id = commitInfo.GetID()
	if util.IdToString(id) != util.IdToString(commitInfo.GetID()) {
		t.Fatal("assertEquals(id, commitInfo.getId())")
	}
	if util.IdToString(id) != util.IdToString(clone.GetID()) {
		t.Fatal("assertEquals(id, clone.getId())")
	}

	commitInfo.AdvanceFieldInfosGen()
	if util.IdToString(id) == util.IdToString(commitInfo.GetID()) {
		t.Fatal("assertNotEquals after advanceFieldInfosGen")
	}
	if util.IdToString(id) != util.IdToString(clone.GetID()) {
		t.Fatal("clone changed but shouldn't")
	}
}

// TestSegmentInfosBitFlippedTriggersCorruptIndexException ports
// testBitFlippedTriggersCorruptIndexException, which commits a SegmentInfos
// and copies the directory with Directory#copyFrom and ExtrasFS#isExtra.
func TestSegmentInfosBitFlippedTriggersCorruptIndexException(t *testing.T) {
	t.Fatal(segmentInfosCommitMissing + "; org.apache.lucene.store.Directory#copyFrom and " +
		"org.apache.lucene.tests.mockfile.ExtrasFS are not ported")
}

func TestSegmentInfosAddDiagnostics(t *testing.T) {
	dir := newDirectory()
	codec := GetDefaultCodec()

	// diagnostics map
	diagnostics := map[string]string{"key1": "value1", "key2": "value2"}

	// adds an additional key/value pair
	si := segmentInfosTestSegmentInfo(t, dir, util.Latest.String(), util.Latest.String(), "TEST", 10000,
		codec, diagnostics, util.RandomId(), map[string]string{}, segmentInfosIndexOrder())
	si.AddDiagnostics(map[string]string{"key3": "value3"})
	if want := map[string]string{"key1": "value1", "key2": "value2", "key3": "value3"}; !maps.Equal(want, si.GetDiagnostics()) {
		t.Fatalf("expected %v, got %v", want, si.GetDiagnostics())
	}

	// modifies an existing key/value pair
	si = segmentInfosTestSegmentInfo(t, dir, util.Latest.String(), util.Latest.String(), "TEST", 10000,
		codec, diagnostics, util.RandomId(), map[string]string{}, segmentInfosIndexOrder())
	si.AddDiagnostics(map[string]string{"key2": "foo"})
	if want := map[string]string{"key1": "value1", "key2": "foo"}; !maps.Equal(want, si.GetDiagnostics()) {
		t.Fatalf("expected %v, got %v", want, si.GetDiagnostics())
	}

	if err := dir.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

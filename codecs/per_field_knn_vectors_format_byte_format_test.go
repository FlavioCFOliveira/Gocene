// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// recordingKnnVectorsFormat is a minimal KnnVectorsFormat that captures
// the SegmentWriteState passed at writer construction, every WriteField
// call it serves, and the SegmentReadState passed at reader construction.
type recordingKnnVectorsFormat struct {
	name        string
	writeStates []*SegmentWriteState
	readStates  []*SegmentReadState
	writers     []*recordingKnnVectorsWriter
	readers     []*recordingKnnVectorsReader
}

func newRecordingKnnVectorsFormat(name string) *recordingKnnVectorsFormat {
	return &recordingKnnVectorsFormat{name: name}
}

func (f *recordingKnnVectorsFormat) Name() string { return f.name }

func (f *recordingKnnVectorsFormat) FieldsWriter(state *SegmentWriteState) (KnnVectorsWriter, error) {
	f.writeStates = append(f.writeStates, state)
	w := &recordingKnnVectorsWriter{format: f, state: state}
	f.writers = append(f.writers, w)
	return w, nil
}

func (f *recordingKnnVectorsFormat) FieldsReader(state *SegmentReadState) (KnnVectorsReader, error) {
	f.readStates = append(f.readStates, state)
	r := &recordingKnnVectorsReader{format: f, state: state}
	f.readers = append(f.readers, r)
	return r, nil
}

type recordingKnnVectorsWriter struct {
	format        *recordingKnnVectorsFormat
	state         *SegmentWriteState
	writtenFields []string
	finished      bool
	closed        bool
}

func (w *recordingKnnVectorsWriter) WriteField(fi *index.FieldInfo, _ KnnVectorsReader) error {
	w.writtenFields = append(w.writtenFields, fi.Name())
	return nil
}
func (w *recordingKnnVectorsWriter) Finish() error {
	w.finished = true
	return nil
}
func (w *recordingKnnVectorsWriter) Close() error {
	w.closed = true
	return nil
}

type recordingKnnVectorsReader struct {
	format *recordingKnnVectorsFormat
	state  *SegmentReadState
	closed bool
}

func (r *recordingKnnVectorsReader) CheckIntegrity() error { return nil }
func (r *recordingKnnVectorsReader) Close() error          { r.closed = true; return nil }

// newVectorFieldInfo creates a frozen FieldInfo carrying a positive
// vector dimension so it qualifies for PerFieldKnnVectorsFormat purposes.
func newVectorFieldInfo(name string, number, dim int) *index.FieldInfo {
	return index.NewFieldInfo(name, number, index.FieldInfoOptions{
		VectorDimension:          dim,
		VectorEncoding:           index.VectorEncodingFloat32,
		VectorSimilarityFunction: index.VectorSimilarityFunctionEuclidean,
	})
}

// TestPerFieldKnnVectorsFormat_SuffixAssignment verifies that two fields
// resolving to the same delegate format share a single underlying writer
// and the same integer suffix (0), and that the FieldInfo attributes
// stamped on each field carry the delegate format name and that shared
// suffix.
func TestPerFieldKnnVectorsFormat_SuffixAssignment(t *testing.T) {
	format := newRecordingKnnVectorsFormat("Lucene99HnswVectorsFormat")

	fis := index.NewFieldInfos()
	for i, name := range []string{"v1", "v2"} {
		if err := fis.Add(newVectorFieldInfo(name, i, 8)); err != nil {
			t.Fatalf("fis.Add(%q): %v", name, err)
		}
	}

	provider := FieldKnnVectorsFormatProviderFunc(func(string) KnnVectorsFormat { return format })
	pf := NewPerFieldKnnVectorsFormat(provider)

	ws, _, dir := newSegmentStates(t, fis)
	defer dir.Close()

	writer, err := pf.FieldsWriter(ws)
	if err != nil {
		t.Fatalf("FieldsWriter: %v", err)
	}
	for _, name := range []string{"v1", "v2"} {
		if err := writer.WriteField(fis.GetByName(name), nil); err != nil {
			t.Fatalf("WriteField(%q): %v", name, err)
		}
	}
	if err := writer.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if got := len(format.writers); got != 1 {
		t.Fatalf("delegate writers: got %d, want 1", got)
	}
	if want := perFieldKnnVectorsSuffix("Lucene99HnswVectorsFormat", "0"); format.writers[0].state.SegmentSuffix != want {
		t.Errorf("delegate SegmentSuffix: got %q, want %q",
			format.writers[0].state.SegmentSuffix, want)
	}
	if got := format.writers[0].writtenFields; len(got) != 2 || got[0] != "v1" || got[1] != "v2" {
		t.Errorf("delegate.writtenFields: got %v, want [v1 v2]", got)
	}
	if !format.writers[0].finished {
		t.Error("delegate writer was not finished")
	}
	if !format.writers[0].closed {
		t.Error("delegate writer was not closed")
	}

	for _, name := range []string{"v1", "v2"} {
		fi := fis.GetByName(name)
		if got := fi.GetAttribute(PER_FIELD_KNN_VECTORS_FORMAT_KEY); got != "Lucene99HnswVectorsFormat" {
			t.Errorf("field %q: format attribute = %q", name, got)
		}
		if got := fi.GetAttribute(PER_FIELD_KNN_VECTORS_SUFFIX_KEY); got != "0" {
			t.Errorf("field %q: suffix attribute = %q", name, got)
		}
	}
}

// TestPerFieldKnnVectorsFormat_DistinctFormatsBumpSuffix verifies that two
// fields resolving to different delegate KnnVectorsFormats produce two
// underlying writers, each carrying its own "<formatName>_<n>" segment
// suffix.
func TestPerFieldKnnVectorsFormat_DistinctFormatsBumpSuffix(t *testing.T) {
	hnsw := newRecordingKnnVectorsFormat("HnswVF")
	flat := newRecordingKnnVectorsFormat("FlatVF")

	fis := index.NewFieldInfos()
	for i, name := range []string{"a", "b", "c"} {
		if err := fis.Add(newVectorFieldInfo(name, i, 8)); err != nil {
			t.Fatalf("fis.Add(%q): %v", name, err)
		}
	}

	provider := FieldKnnVectorsFormatProviderFunc(func(field string) KnnVectorsFormat {
		if field == "b" {
			return flat
		}
		return hnsw
	})
	pf := NewPerFieldKnnVectorsFormat(provider)

	ws, _, dir := newSegmentStates(t, fis)
	defer dir.Close()

	writer, err := pf.FieldsWriter(ws)
	if err != nil {
		t.Fatalf("FieldsWriter: %v", err)
	}
	for _, name := range []string{"a", "b", "c"} {
		if err := writer.WriteField(fis.GetByName(name), nil); err != nil {
			t.Fatalf("WriteField(%q): %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if got := len(hnsw.writers); got != 1 {
		t.Fatalf("HnswVF writers: got %d, want 1", got)
	}
	if got := len(flat.writers); got != 1 {
		t.Fatalf("FlatVF writers: got %d, want 1", got)
	}

	wantSuffix := map[string]struct {
		formatName, suffix string
	}{
		"a": {"HnswVF", "0"},
		"b": {"FlatVF", "0"},
		"c": {"HnswVF", "0"},
	}
	for name, want := range wantSuffix {
		fi := fis.GetByName(name)
		if got := fi.GetAttribute(PER_FIELD_KNN_VECTORS_FORMAT_KEY); got != want.formatName {
			t.Errorf("field %q: format attribute = %q, want %q", name, got, want.formatName)
		}
		if got := fi.GetAttribute(PER_FIELD_KNN_VECTORS_SUFFIX_KEY); got != want.suffix {
			t.Errorf("field %q: suffix attribute = %q, want %q", name, got, want.suffix)
		}
	}
}

// TestPerFieldKnnVectorsFormat_BumpSuffixPerFormatName verifies that two
// *different* KnnVectorsFormat instances that share the same name receive
// distinct writers, with the suffix counter advancing from 0 to 1.
func TestPerFieldKnnVectorsFormat_BumpSuffixPerFormatName(t *testing.T) {
	const name = "Lucene99HnswVectorsFormat"
	first := newRecordingKnnVectorsFormat(name)
	second := newRecordingKnnVectorsFormat(name)

	fis := index.NewFieldInfos()
	for i, fname := range []string{"a", "b"} {
		if err := fis.Add(newVectorFieldInfo(fname, i, 8)); err != nil {
			t.Fatalf("fis.Add(%q): %v", fname, err)
		}
	}

	provider := FieldKnnVectorsFormatProviderFunc(func(field string) KnnVectorsFormat {
		if field == "a" {
			return first
		}
		return second
	})
	pf := NewPerFieldKnnVectorsFormat(provider)

	ws, _, dir := newSegmentStates(t, fis)
	defer dir.Close()

	writer, err := pf.FieldsWriter(ws)
	if err != nil {
		t.Fatalf("FieldsWriter: %v", err)
	}
	for _, fname := range []string{"a", "b"} {
		if err := writer.WriteField(fis.GetByName(fname), nil); err != nil {
			t.Fatalf("WriteField(%q): %v", fname, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if got := fis.GetByName("a").GetAttribute(PER_FIELD_KNN_VECTORS_SUFFIX_KEY); got != "0" {
		t.Errorf("field a suffix = %q, want %q", got, "0")
	}
	if got := fis.GetByName("b").GetAttribute(PER_FIELD_KNN_VECTORS_SUFFIX_KEY); got != "1" {
		t.Errorf("field b suffix = %q, want %q", got, "1")
	}
	if want := perFieldKnnVectorsSuffix(name, "0"); first.writers[0].state.SegmentSuffix != want {
		t.Errorf("first SegmentSuffix: got %q, want %q",
			first.writers[0].state.SegmentSuffix, want)
	}
	if want := perFieldKnnVectorsSuffix(name, "1"); second.writers[0].state.SegmentSuffix != want {
		t.Errorf("second SegmentSuffix: got %q, want %q",
			second.writers[0].state.SegmentSuffix, want)
	}
}

// TestPerFieldKnnVectorsFormat_ReaderDispatch verifies that the reader,
// given only FieldInfo attributes and a registered KnnVectorsFormat,
// opens one delegate KnnVectorsReader per "<formatName>_<n>" suffix and
// routes GetFieldReader(field) to the reader it shares with that field.
func TestPerFieldKnnVectorsFormat_ReaderDispatch(t *testing.T) {
	hnsw := newRecordingKnnVectorsFormat("HnswVF")
	flat := newRecordingKnnVectorsFormat("FlatVF")

	RegisterKnnVectorsFormat(hnsw)
	RegisterKnnVectorsFormat(flat)
	t.Cleanup(func() {
		UnregisterKnnVectorsFormat("HnswVF")
		UnregisterKnnVectorsFormat("FlatVF")
	})

	fis := index.NewFieldInfos()
	type entry struct {
		name, format, suffix string
	}
	for i, e := range []entry{
		{"a", "HnswVF", "0"},
		{"b", "FlatVF", "0"},
		{"c", "HnswVF", "0"},
	} {
		fi := newVectorFieldInfo(e.name, i, 8)
		fi.PutCodecAttribute(PER_FIELD_KNN_VECTORS_FORMAT_KEY, e.format)
		fi.PutCodecAttribute(PER_FIELD_KNN_VECTORS_SUFFIX_KEY, e.suffix)
		if err := fis.Add(fi); err != nil {
			t.Fatalf("fis.Add(%q): %v", e.name, err)
		}
	}

	_, rs, dir := newSegmentStates(t, fis)
	defer dir.Close()

	pf := NewPerFieldKnnVectorsFormat(nil)
	reader, err := pf.FieldsReader(rs)
	if err != nil {
		t.Fatalf("FieldsReader: %v", err)
	}

	if got := len(hnsw.readers); got != 1 {
		t.Errorf("HnswVF readers: got %d, want 1", got)
	}
	if got := len(flat.readers); got != 1 {
		t.Errorf("FlatVF readers: got %d, want 1", got)
	}

	// GetFieldReader should dispatch to the same delegate for fields that
	// share its suffix.
	pf2 := reader.(*PerFieldKnnVectorsReader)
	if got := pf2.GetFieldReader("a"); got == nil {
		t.Error("GetFieldReader(a) returned nil")
	} else if got != pf2.GetFieldReader("c") {
		t.Error("GetFieldReader(a) and GetFieldReader(c) should share a delegate")
	}
	if got := pf2.GetFieldReader("b"); got == nil {
		t.Error("GetFieldReader(b) returned nil")
	} else if got == pf2.GetFieldReader("a") {
		t.Error("GetFieldReader(a) and GetFieldReader(b) should not share a delegate")
	}
	if got := pf2.GetFieldReader("missing"); got != nil {
		t.Errorf("GetFieldReader(missing) = %v, want nil", got)
	}

	if err := reader.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !hnsw.readers[0].closed {
		t.Error("HnswVF reader was not closed")
	}
	if !flat.readers[0].closed {
		t.Error("FlatVF reader was not closed")
	}
}

// TestPerFieldKnnVectorsFormat_ReaderMissingSuffixAttribute confirms that
// the reader rejects a FieldInfo carrying a format-name but no suffix
// attribute.
func TestPerFieldKnnVectorsFormat_ReaderMissingSuffixAttribute(t *testing.T) {
	format := newRecordingKnnVectorsFormat("XVF")
	RegisterKnnVectorsFormat(format)
	t.Cleanup(func() { UnregisterKnnVectorsFormat("XVF") })

	fis := index.NewFieldInfos()
	fi := newVectorFieldInfo("a", 0, 8)
	fi.PutCodecAttribute(PER_FIELD_KNN_VECTORS_FORMAT_KEY, "XVF")
	// suffix attribute intentionally missing
	if err := fis.Add(fi); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	_, rs, dir := newSegmentStates(t, fis)
	defer dir.Close()

	pf := NewPerFieldKnnVectorsFormat(nil)
	if _, err := pf.FieldsReader(rs); err == nil {
		t.Fatal("FieldsReader succeeded; want error about missing suffix")
	}
}

// TestPerFieldKnnVectorsFormat_ReaderUnknownFormatName verifies that the
// reader surfaces a registry miss as an error rather than returning a
// half-built reader.
func TestPerFieldKnnVectorsFormat_ReaderUnknownFormatName(t *testing.T) {
	fis := index.NewFieldInfos()
	fi := newVectorFieldInfo("a", 0, 8)
	fi.PutCodecAttribute(PER_FIELD_KNN_VECTORS_FORMAT_KEY, "UnknownVF")
	fi.PutCodecAttribute(PER_FIELD_KNN_VECTORS_SUFFIX_KEY, "0")
	if err := fis.Add(fi); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	_, rs, dir := newSegmentStates(t, fis)
	defer dir.Close()

	pf := NewPerFieldKnnVectorsFormat(nil)
	if _, err := pf.FieldsReader(rs); err == nil {
		t.Fatal("FieldsReader succeeded; want error about unknown format name")
	}
}

// TestPerFieldKnnVectorsFormat_NonVectorFieldsIgnored verifies that the
// reader skips FieldInfos whose VectorDimension is zero, mirroring Java's
// hasVectorValues guard.
func TestPerFieldKnnVectorsFormat_NonVectorFieldsIgnored(t *testing.T) {
	format := newRecordingKnnVectorsFormat("VF1")
	RegisterKnnVectorsFormat(format)
	t.Cleanup(func() { UnregisterKnnVectorsFormat("VF1") })

	fis := index.NewFieldInfos()
	// Indexed-only field, no vectors, no codec attributes.
	if err := fis.Add(index.NewFieldInfo("indexed_only", 0, index.FieldInfoOptions{
		IndexOptions: index.IndexOptionsDocs,
	})); err != nil {
		t.Fatalf("fis.Add(indexed_only): %v", err)
	}
	// Vector field with PerField metadata.
	v := newVectorFieldInfo("v", 1, 8)
	v.PutCodecAttribute(PER_FIELD_KNN_VECTORS_FORMAT_KEY, "VF1")
	v.PutCodecAttribute(PER_FIELD_KNN_VECTORS_SUFFIX_KEY, "0")
	if err := fis.Add(v); err != nil {
		t.Fatalf("fis.Add(v): %v", err)
	}

	_, rs, dir := newSegmentStates(t, fis)
	defer dir.Close()

	pf := NewPerFieldKnnVectorsFormat(nil)
	reader, err := pf.FieldsReader(rs)
	if err != nil {
		t.Fatalf("FieldsReader: %v", err)
	}
	defer reader.Close()

	pf2 := reader.(*PerFieldKnnVectorsReader)
	if got := pf2.GetFieldReader("indexed_only"); got != nil {
		t.Errorf("GetFieldReader(indexed_only) = %v, want nil", got)
	}
	if got := pf2.GetFieldReader("v"); got == nil {
		t.Error("GetFieldReader(v) returned nil")
	}
}

// TestPerFieldKnnVectorsFormat_RegistryRoundTrip is a focused sanity
// check on the new package-level KnnVectorsFormat registry.
func TestPerFieldKnnVectorsFormat_RegistryRoundTrip(t *testing.T) {
	const name = "RoundTripVF"
	format := newRecordingKnnVectorsFormat(name)

	if _, err := KnnVectorsFormatByName(name); err == nil {
		t.Fatal("KnnVectorsFormatByName succeeded before registration")
	}

	RegisterKnnVectorsFormat(format)
	got, err := KnnVectorsFormatByName(name)
	if err != nil {
		t.Fatalf("KnnVectorsFormatByName after register: %v", err)
	}
	if got != format {
		t.Errorf("KnnVectorsFormatByName returned %v, want the registered instance", got)
	}

	UnregisterKnnVectorsFormat(name)
	if _, err := KnnVectorsFormatByName(name); err == nil {
		t.Fatal("KnnVectorsFormatByName succeeded after unregister")
	}
}

// TestPerFieldKnnVectorsFormat_SuffixFormat documents the exact
// "<name>_<n>" shape that ends up on disk; the byte upgrade hinges on
// this.
func TestPerFieldKnnVectorsFormat_SuffixFormat(t *testing.T) {
	for _, tc := range []struct {
		formatName string
		suffix     int
		want       string
	}{
		{"Lucene99HnswVectorsFormat", 0, "Lucene99HnswVectorsFormat_0"},
		{"Lucene99HnswVectorsFormat", 1, "Lucene99HnswVectorsFormat_1"},
		{"X", 12, "X_12"},
	} {
		got := perFieldKnnVectorsSuffix(tc.formatName, strconv.Itoa(tc.suffix))
		if got != tc.want {
			t.Errorf("perFieldKnnVectorsSuffix(%q, %d) = %q, want %q",
				tc.formatName, tc.suffix, got, tc.want)
		}
	}
}

// TestPerFieldKnnVectorsFormat_FullSegmentSuffix verifies the KNN
// nesting behaviour: outer suffix prepended with "_" when non-empty.
func TestPerFieldKnnVectorsFormat_FullSegmentSuffix(t *testing.T) {
	for _, tc := range []struct {
		outer, inner, want string
	}{
		{"", "HnswVF_0", "HnswVF_0"},
		{"outer", "HnswVF_0", "outer_HnswVF_0"},
	} {
		got := perFieldKnnVectorsFullSegmentSuffix(tc.outer, tc.inner)
		if got != tc.want {
			t.Errorf("perFieldKnnVectorsFullSegmentSuffix(%q,%q) = %q, want %q",
				tc.outer, tc.inner, got, tc.want)
		}
	}
}

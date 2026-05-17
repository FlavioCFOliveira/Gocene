// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// recordingPostingsFormat is a minimal PostingsFormat used to assert the
// FieldsConsumer/FieldsProducer that the per-field writer/reader created
// for it, and to record the SegmentSuffix it was opened with.
type recordingPostingsFormat struct {
	name              string
	writeStates       []*SegmentWriteState
	readStates        []*SegmentReadState
	consumers         []*recordingFieldsConsumer
	producers         []*recordingFieldsProducer
	fieldsForProducer map[string][]string
}

func newRecordingPostingsFormat(name string) *recordingPostingsFormat {
	return &recordingPostingsFormat{
		name:              name,
		fieldsForProducer: make(map[string][]string),
	}
}

func (f *recordingPostingsFormat) Name() string { return f.name }

func (f *recordingPostingsFormat) FieldsConsumer(state *SegmentWriteState) (FieldsConsumer, error) {
	f.writeStates = append(f.writeStates, state)
	c := &recordingFieldsConsumer{format: f, state: state}
	f.consumers = append(f.consumers, c)
	return c, nil
}

func (f *recordingPostingsFormat) FieldsProducer(state *SegmentReadState) (FieldsProducer, error) {
	f.readStates = append(f.readStates, state)
	p := &recordingFieldsProducer{
		format: f,
		state:  state,
		terms:  make(map[string]index.Terms),
	}
	// Seed the producer with empty terms for every field configured for
	// this segmentSuffix; this lets the dispatch test assert that the
	// returned Terms is the same instance produced here.
	for _, field := range f.fieldsForProducer[state.SegmentSuffix] {
		p.terms[field] = &emptyTerms{}
	}
	f.producers = append(f.producers, p)
	return p, nil
}

type recordingFieldsConsumer struct {
	format       *recordingPostingsFormat
	state        *SegmentWriteState
	writtenField []string
	closed       bool
}

func (c *recordingFieldsConsumer) Write(field string, terms index.Terms) error {
	c.writtenField = append(c.writtenField, field)
	return nil
}

func (c *recordingFieldsConsumer) Close() error {
	c.closed = true
	return nil
}

type recordingFieldsProducer struct {
	format *recordingPostingsFormat
	state  *SegmentReadState
	terms  map[string]index.Terms
	closed bool
}

func (p *recordingFieldsProducer) Terms(field string) (index.Terms, error) {
	t, ok := p.terms[field]
	if !ok {
		return nil, nil
	}
	return t, nil
}

func (p *recordingFieldsProducer) Close() error {
	p.closed = true
	return nil
}

// newIndexedFieldInfo creates a frozen FieldInfo with DOCS index options so
// it qualifies as an indexed field for PerFieldPostingsFormat purposes.
func newIndexedFieldInfo(name string, number int) *index.FieldInfo {
	return index.NewFieldInfo(name, number, index.FieldInfoOptions{
		IndexOptions: index.IndexOptionsDocs,
	})
}

// newSegmentStates returns a paired (write, read) state on top of a fresh
// in-memory directory for use by per-field writer/reader tests. The caller
// owns the directory and must close it.
func newSegmentStates(t *testing.T, fis *index.FieldInfos) (*SegmentWriteState, *SegmentReadState, store.Directory) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	si := index.NewSegmentInfo("_0", 0, dir)
	ws := &SegmentWriteState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  fis,
	}
	rs := &SegmentReadState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  fis,
	}
	return ws, rs, dir
}

// TestPerFieldPostingsFormat_SuffixAssignment verifies that two fields that
// resolve to the same delegate format share a single underlying consumer and
// the same integer suffix (0), and that the FieldInfo attributes stamped on
// each field carry the delegate format name and that shared suffix.
func TestPerFieldPostingsFormat_SuffixAssignment(t *testing.T) {
	format := newRecordingPostingsFormat("Lucene104PostingsFormat")

	fis := index.NewFieldInfos()
	for i, name := range []string{"a", "b"} {
		if err := fis.Add(newIndexedFieldInfo(name, i)); err != nil {
			t.Fatalf("fis.Add(%q): %v", name, err)
		}
	}

	provider := FieldPostingsFormatProviderFunc(func(string) PostingsFormat { return format })
	pf := NewPerFieldPostingsFormat(provider)

	ws, _, dir := newSegmentStates(t, fis)
	defer dir.Close()

	consumer, err := pf.FieldsConsumer(ws)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}

	for _, name := range []string{"a", "b"} {
		if err := consumer.Write(name, &emptyTerms{}); err != nil {
			t.Fatalf("Write(%q): %v", name, err)
		}
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// One delegate format instance must produce exactly one underlying
	// FieldsConsumer, regardless of how many fields it serves.
	if got := len(format.consumers); got != 1 {
		t.Fatalf("delegate consumers: got %d, want 1", got)
	}
	if want := perFieldPostingsSuffix("Lucene104PostingsFormat", "0"); format.consumers[0].state.SegmentSuffix != want {
		t.Errorf("delegate SegmentSuffix: got %q, want %q",
			format.consumers[0].state.SegmentSuffix, want)
	}
	if got := format.consumers[0].writtenField; len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("delegate.writtenField: got %v, want [a b]", got)
	}

	for _, name := range []string{"a", "b"} {
		fi := fis.GetByName(name)
		if got := fi.GetAttribute(PER_FIELD_POSTINGS_FORMAT_KEY); got != "Lucene104PostingsFormat" {
			t.Errorf("field %q: format attribute = %q, want %q",
				name, got, "Lucene104PostingsFormat")
		}
		if got := fi.GetAttribute(PER_FIELD_POSTINGS_SUFFIX_KEY); got != "0" {
			t.Errorf("field %q: suffix attribute = %q, want %q", name, got, "0")
		}
	}
}

// TestPerFieldPostingsFormat_DistinctFormatsBumpSuffix verifies that two
// fields resolving to different delegate formats produce two underlying
// consumers, each carrying its own "<formatName>_<n>" segment suffix, and
// that suffix counters are scoped per format-name.
func TestPerFieldPostingsFormat_DistinctFormatsBumpSuffix(t *testing.T) {
	fast := newRecordingPostingsFormat("Fast")
	slow := newRecordingPostingsFormat("Slow")

	fis := index.NewFieldInfos()
	for i, name := range []string{"a", "b", "c"} {
		if err := fis.Add(newIndexedFieldInfo(name, i)); err != nil {
			t.Fatalf("fis.Add(%q): %v", name, err)
		}
	}

	provider := FieldPostingsFormatProviderFunc(func(field string) PostingsFormat {
		if field == "b" {
			return slow
		}
		return fast
	})
	pf := NewPerFieldPostingsFormat(provider)

	ws, _, dir := newSegmentStates(t, fis)
	defer dir.Close()

	consumer, err := pf.FieldsConsumer(ws)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}
	for _, name := range []string{"a", "b", "c"} {
		if err := consumer.Write(name, &emptyTerms{}); err != nil {
			t.Fatalf("Write(%q): %v", name, err)
		}
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if got := len(fast.consumers); got != 1 {
		t.Fatalf("fast delegate consumers: got %d, want 1", got)
	}
	if got := len(slow.consumers); got != 1 {
		t.Fatalf("slow delegate consumers: got %d, want 1", got)
	}
	if want := perFieldPostingsSuffix("Fast", "0"); fast.consumers[0].state.SegmentSuffix != want {
		t.Errorf("fast SegmentSuffix: got %q, want %q",
			fast.consumers[0].state.SegmentSuffix, want)
	}
	if want := perFieldPostingsSuffix("Slow", "0"); slow.consumers[0].state.SegmentSuffix != want {
		t.Errorf("slow SegmentSuffix: got %q, want %q",
			slow.consumers[0].state.SegmentSuffix, want)
	}

	wantSuffix := map[string]struct {
		formatName, suffix string
	}{
		"a": {"Fast", "0"},
		"b": {"Slow", "0"},
		"c": {"Fast", "0"},
	}
	for name, want := range wantSuffix {
		fi := fis.GetByName(name)
		if got := fi.GetAttribute(PER_FIELD_POSTINGS_FORMAT_KEY); got != want.formatName {
			t.Errorf("field %q: format attribute = %q, want %q", name, got, want.formatName)
		}
		if got := fi.GetAttribute(PER_FIELD_POSTINGS_SUFFIX_KEY); got != want.suffix {
			t.Errorf("field %q: suffix attribute = %q, want %q", name, got, want.suffix)
		}
	}
}

// TestPerFieldPostingsFormat_BumpSuffixPerFormatName verifies that when the
// provider returns two *different* PostingsFormat instances that share the
// same format name (Java's "first time we are seeing this format" case),
// each instance receives its own consumer and the suffix counter advances
// from 0 to 1 within that format-name scope.
func TestPerFieldPostingsFormat_BumpSuffixPerFormatName(t *testing.T) {
	const name = "Lucene104PostingsFormat"
	first := newRecordingPostingsFormat(name)
	second := newRecordingPostingsFormat(name)

	fis := index.NewFieldInfos()
	for i, fname := range []string{"a", "b"} {
		if err := fis.Add(newIndexedFieldInfo(fname, i)); err != nil {
			t.Fatalf("fis.Add(%q): %v", fname, err)
		}
	}

	provider := FieldPostingsFormatProviderFunc(func(field string) PostingsFormat {
		if field == "a" {
			return first
		}
		return second
	})
	pf := NewPerFieldPostingsFormat(provider)

	ws, _, dir := newSegmentStates(t, fis)
	defer dir.Close()

	consumer, err := pf.FieldsConsumer(ws)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}
	for _, fname := range []string{"a", "b"} {
		if err := consumer.Write(fname, &emptyTerms{}); err != nil {
			t.Fatalf("Write(%q): %v", fname, err)
		}
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if got := fis.GetByName("a").GetAttribute(PER_FIELD_POSTINGS_SUFFIX_KEY); got != "0" {
		t.Errorf("field a suffix = %q, want %q", got, "0")
	}
	if got := fis.GetByName("b").GetAttribute(PER_FIELD_POSTINGS_SUFFIX_KEY); got != "1" {
		t.Errorf("field b suffix = %q, want %q", got, "1")
	}
	if want := perFieldPostingsSuffix(name, "0"); first.consumers[0].state.SegmentSuffix != want {
		t.Errorf("first SegmentSuffix: got %q, want %q",
			first.consumers[0].state.SegmentSuffix, want)
	}
	if want := perFieldPostingsSuffix(name, "1"); second.consumers[0].state.SegmentSuffix != want {
		t.Errorf("second SegmentSuffix: got %q, want %q",
			second.consumers[0].state.SegmentSuffix, want)
	}
}

// TestPerFieldPostingsFormat_ReaderDispatch verifies that the reader, given
// only FieldInfo attributes and a registered PostingsFormat, opens one
// delegate FieldsProducer per "<formatName>_<n>" suffix and routes each
// Terms(field) call to the producer it shares with that field.
func TestPerFieldPostingsFormat_ReaderDispatch(t *testing.T) {
	fast := newRecordingPostingsFormat("FastPF")
	slow := newRecordingPostingsFormat("SlowPF")

	// Both fields for FastPF share the same suffix (0); SlowPF has one.
	fast.fieldsForProducer[perFieldPostingsSuffix("FastPF", "0")] = []string{"a", "c"}
	slow.fieldsForProducer[perFieldPostingsSuffix("SlowPF", "0")] = []string{"b"}

	RegisterPostingsFormat(fast)
	RegisterPostingsFormat(slow)
	t.Cleanup(func() {
		UnregisterPostingsFormat("FastPF")
		UnregisterPostingsFormat("SlowPF")
	})

	fis := index.NewFieldInfos()
	type entry struct {
		name, format, suffix string
	}
	entries := []entry{
		{"a", "FastPF", "0"},
		{"b", "SlowPF", "0"},
		{"c", "FastPF", "0"},
	}
	for i, e := range entries {
		fi := newIndexedFieldInfo(e.name, i)
		fi.PutCodecAttribute(PER_FIELD_POSTINGS_FORMAT_KEY, e.format)
		fi.PutCodecAttribute(PER_FIELD_POSTINGS_SUFFIX_KEY, e.suffix)
		if err := fis.Add(fi); err != nil {
			t.Fatalf("fis.Add(%q): %v", e.name, err)
		}
	}

	_, rs, dir := newSegmentStates(t, fis)
	defer dir.Close()

	pf := NewPerFieldPostingsFormat(nil) // formatProvider is unused on the read path
	producer, err := pf.FieldsProducer(rs)
	if err != nil {
		t.Fatalf("FieldsProducer: %v", err)
	}

	// Reader should have opened one delegate producer per format-name suffix:
	// one for FastPF/0 (shared by a + c) and one for SlowPF/0 (b).
	if got := len(fast.producers); got != 1 {
		t.Errorf("FastPF producers: got %d, want 1", got)
	}
	if got := len(slow.producers); got != 1 {
		t.Errorf("SlowPF producers: got %d, want 1", got)
	}
	if want := perFieldPostingsSuffix("FastPF", "0"); fast.producers[0].state.SegmentSuffix != want {
		t.Errorf("FastPF SegmentSuffix: got %q, want %q",
			fast.producers[0].state.SegmentSuffix, want)
	}
	if want := perFieldPostingsSuffix("SlowPF", "0"); slow.producers[0].state.SegmentSuffix != want {
		t.Errorf("SlowPF SegmentSuffix: got %q, want %q",
			slow.producers[0].state.SegmentSuffix, want)
	}

	// Terms() dispatches to the right delegate, even though the reader was
	// constructed without the original formatProvider.
	for _, name := range []string{"a", "b", "c"} {
		terms, err := producer.Terms(name)
		if err != nil {
			t.Fatalf("Terms(%q): %v", name, err)
		}
		if terms == nil {
			t.Errorf("Terms(%q) = nil, want non-nil", name)
		}
	}

	// Unknown field must return (nil, nil) per the Java contract.
	terms, err := producer.Terms("missing")
	if err != nil {
		t.Fatalf("Terms(missing): %v", err)
	}
	if terms != nil {
		t.Errorf("Terms(missing) = %v, want nil", terms)
	}

	if err := producer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !fast.producers[0].closed {
		t.Error("FastPF producer was not closed")
	}
	if !slow.producers[0].closed {
		t.Error("SlowPF producer was not closed")
	}
}

// TestPerFieldPostingsFormat_ReaderMissingSuffixAttribute confirms that the
// reader rejects a FieldInfo carrying a format-name but no suffix attribute,
// matching the IllegalStateException Lucene throws in the same condition.
func TestPerFieldPostingsFormat_ReaderMissingSuffixAttribute(t *testing.T) {
	format := newRecordingPostingsFormat("XPF")
	RegisterPostingsFormat(format)
	t.Cleanup(func() { UnregisterPostingsFormat("XPF") })

	fis := index.NewFieldInfos()
	fi := newIndexedFieldInfo("a", 0)
	fi.PutCodecAttribute(PER_FIELD_POSTINGS_FORMAT_KEY, "XPF")
	// suffix attribute intentionally missing
	if err := fis.Add(fi); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	_, rs, dir := newSegmentStates(t, fis)
	defer dir.Close()

	pf := NewPerFieldPostingsFormat(nil)
	if _, err := pf.FieldsProducer(rs); err == nil {
		t.Fatal("FieldsProducer succeeded; want error about missing suffix")
	}
}

// TestPerFieldPostingsFormat_ReaderUnknownFormatName verifies that the reader
// surfaces a registry miss as an error rather than returning a half-built
// producer.
func TestPerFieldPostingsFormat_ReaderUnknownFormatName(t *testing.T) {
	fis := index.NewFieldInfos()
	fi := newIndexedFieldInfo("a", 0)
	fi.PutCodecAttribute(PER_FIELD_POSTINGS_FORMAT_KEY, "UnknownPF")
	fi.PutCodecAttribute(PER_FIELD_POSTINGS_SUFFIX_KEY, "0")
	if err := fis.Add(fi); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	_, rs, dir := newSegmentStates(t, fis)
	defer dir.Close()

	pf := NewPerFieldPostingsFormat(nil)
	if _, err := pf.FieldsProducer(rs); err == nil {
		t.Fatal("FieldsProducer succeeded; want error about unknown format name")
	}
}

// TestPerFieldPostingsFormat_NonIndexedFieldsIgnored verifies that the reader
// skips FieldInfos that do not have IndexOptions.IsIndexed(): these carry no
// postings and never receive PerField attributes.
func TestPerFieldPostingsFormat_NonIndexedFieldsIgnored(t *testing.T) {
	format := newRecordingPostingsFormat("PF1")
	RegisterPostingsFormat(format)
	t.Cleanup(func() { UnregisterPostingsFormat("PF1") })

	fis := index.NewFieldInfos()
	// Non-indexed field (IndexOptionsNone), no codec attributes.
	if err := fis.Add(index.NewFieldInfo("stored_only", 0, index.FieldInfoOptions{
		Stored: true,
	})); err != nil {
		t.Fatalf("fis.Add(stored_only): %v", err)
	}
	// Indexed field with PerField metadata.
	indexed := newIndexedFieldInfo("indexed", 1)
	indexed.PutCodecAttribute(PER_FIELD_POSTINGS_FORMAT_KEY, "PF1")
	indexed.PutCodecAttribute(PER_FIELD_POSTINGS_SUFFIX_KEY, "0")
	if err := fis.Add(indexed); err != nil {
		t.Fatalf("fis.Add(indexed): %v", err)
	}
	format.fieldsForProducer[perFieldPostingsSuffix("PF1", "0")] = []string{"indexed"}

	_, rs, dir := newSegmentStates(t, fis)
	defer dir.Close()

	pf := NewPerFieldPostingsFormat(nil)
	producer, err := pf.FieldsProducer(rs)
	if err != nil {
		t.Fatalf("FieldsProducer: %v", err)
	}
	defer producer.Close()

	// The non-indexed field has no underlying producer; Terms must return nil.
	terms, err := producer.Terms("stored_only")
	if err != nil {
		t.Fatalf("Terms(stored_only): %v", err)
	}
	if terms != nil {
		t.Errorf("Terms(stored_only) = %v, want nil", terms)
	}
}

// TestPerFieldPostingsFormat_RegistryRoundTrip is a focused sanity check on
// the new package-level PostingsFormat registry.
func TestPerFieldPostingsFormat_RegistryRoundTrip(t *testing.T) {
	const name = "RoundTripPF"
	format := newRecordingPostingsFormat(name)

	if _, err := PostingsFormatByName(name); err == nil {
		t.Fatal("PostingsFormatByName succeeded before registration")
	}

	RegisterPostingsFormat(format)
	got, err := PostingsFormatByName(name)
	if err != nil {
		t.Fatalf("PostingsFormatByName after register: %v", err)
	}
	if got != format {
		t.Errorf("PostingsFormatByName returned %v, want the registered instance", got)
	}

	UnregisterPostingsFormat(name)
	if _, err := PostingsFormatByName(name); err == nil {
		t.Fatal("PostingsFormatByName succeeded after unregister")
	}
}

// TestPerFieldPostingsFormat_SuffixFormat documents the exact "<name>_<n>"
// shape that ends up on disk; the byte upgrade hinges on this.
func TestPerFieldPostingsFormat_SuffixFormat(t *testing.T) {
	for _, tc := range []struct {
		formatName string
		suffix     int
		want       string
	}{
		{"Lucene104PostingsFormat", 0, "Lucene104PostingsFormat_0"},
		{"Lucene104PostingsFormat", 1, "Lucene104PostingsFormat_1"},
		{"X", 12, "X_12"},
	} {
		got := perFieldPostingsSuffix(tc.formatName, strconv.Itoa(tc.suffix))
		if got != tc.want {
			t.Errorf("perFieldPostingsSuffix(%q, %d) = %q, want %q",
				tc.formatName, tc.suffix, got, tc.want)
		}
	}
}

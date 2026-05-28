// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// recordingDocValuesFormat is a minimal DocValuesFormat used to assert the
// per-field writer/reader behaviour: it captures the SegmentWriteState
// passed at consumer construction, every Add*Field call it serves, and the
// SegmentReadState passed at producer construction.
type recordingDocValuesFormat struct {
	name            string
	writeStates     []*SegmentWriteState
	readStates      []*SegmentReadState
	consumers       []*recordingDocValuesConsumer
	producers       []*recordingDocValuesProducer
	fieldsPerSuffix map[string][]int // field-number routing for producer dispatch
}

func newRecordingDocValuesFormat(name string) *recordingDocValuesFormat {
	return &recordingDocValuesFormat{
		name:            name,
		fieldsPerSuffix: make(map[string][]int),
	}
}

func (f *recordingDocValuesFormat) Name() string { return f.name }

func (f *recordingDocValuesFormat) FieldsConsumer(state *SegmentWriteState) (DocValuesConsumer, error) {
	f.writeStates = append(f.writeStates, state)
	c := &recordingDocValuesConsumer{format: f, state: state}
	f.consumers = append(f.consumers, c)
	return c, nil
}

func (f *recordingDocValuesFormat) FieldsProducer(state *SegmentReadState) (DocValuesProducer, error) {
	f.readStates = append(f.readStates, state)
	p := &recordingDocValuesProducer{
		format:  f,
		state:   state,
		serving: make(map[int]bool),
	}
	for _, fieldNum := range f.fieldsPerSuffix[state.SegmentSuffix] {
		p.serving[fieldNum] = true
	}
	f.producers = append(f.producers, p)
	return p, nil
}

type recordingDocValuesConsumer struct {
	format       *recordingDocValuesFormat
	state        *SegmentWriteState
	addedNumeric []string
	addedBinary  []string
	closed       bool
}

func (c *recordingDocValuesConsumer) AddNumericField(field *index.FieldInfo, _ NumericDocValuesIterator) error {
	c.addedNumeric = append(c.addedNumeric, field.Name())
	return nil
}

func (c *recordingDocValuesConsumer) AddBinaryField(field *index.FieldInfo, _ BinaryDocValuesIterator) error {
	c.addedBinary = append(c.addedBinary, field.Name())
	return nil
}

func (c *recordingDocValuesConsumer) AddSortedField(field *index.FieldInfo, _ SortedDocValuesIterator) error {
	return nil
}
func (c *recordingDocValuesConsumer) AddSortedSetField(*index.FieldInfo, SortedSetDocValuesIterator) error {
	return nil
}
func (c *recordingDocValuesConsumer) AddSortedNumericField(*index.FieldInfo, SortedNumericDocValuesIterator) error {
	return nil
}
func (c *recordingDocValuesConsumer) Close() error {
	c.closed = true
	return nil
}

type recordingDocValuesProducer struct {
	format  *recordingDocValuesFormat
	state   *SegmentReadState
	serving map[int]bool
	closed  bool
}

func (p *recordingDocValuesProducer) GetNumeric(field *index.FieldInfo) (NumericDocValues, error) {
	if !p.serving[field.Number()] {
		return nil, nil
	}
	return numericMarker{n: field.Number()}, nil
}

func (p *recordingDocValuesProducer) GetBinary(*index.FieldInfo) (BinaryDocValues, error) {
	return nil, nil
}
func (p *recordingDocValuesProducer) GetSorted(*index.FieldInfo) (SortedDocValues, error) {
	return nil, nil
}
func (p *recordingDocValuesProducer) GetSortedSet(*index.FieldInfo) (SortedSetDocValues, error) {
	return nil, nil
}
func (p *recordingDocValuesProducer) GetSortedNumeric(*index.FieldInfo) (SortedNumericDocValues, error) {
	return nil, nil
}
func (p *recordingDocValuesProducer) GetSkipper(*index.FieldInfo) (DocValuesSkipper, error) {
	return nil, nil
}
func (p *recordingDocValuesProducer) CheckIntegrity() error { return nil }
func (p *recordingDocValuesProducer) Close() error          { p.closed = true; return nil }

// numericMarker is a sentinel NumericDocValues used solely to assert that
// a non-nil value travels through the per-field reader on dispatch.
type numericMarker struct{ n int }

func (numericMarker) DocID() int                  { return -1 }
func (numericMarker) NextDoc() (int, error)       { return index.NO_MORE_DOCS, nil }
func (numericMarker) Advance(int) (int, error)    { return index.NO_MORE_DOCS, nil }
func (m numericMarker) LongValue() (int64, error) { return int64(m.n), nil }
func (numericMarker) Cost() int64                 { return 0 }

// newNumericFieldInfo creates a frozen FieldInfo with NUMERIC doc-values so
// it qualifies for PerFieldDocValuesFormat purposes.
func newNumericFieldInfo(name string, number int) *index.FieldInfo {
	return index.NewFieldInfo(name, number, index.FieldInfoOptions{
		DocValuesType: index.DocValuesTypeNumeric,
		DocValuesGen:  -1,
	})
}

// nopNumericIterator is a NumericDocValuesIterator that exposes no values;
// it is sufficient to drive the per-field writer's getInstance side effects.
type nopNumericIterator struct{}

func (nopNumericIterator) Next() bool   { return false }
func (nopNumericIterator) DocID() int   { return -1 }
func (nopNumericIterator) Value() int64 { return 0 }

// TestPerFieldDocValuesFormat_SuffixAssignment verifies that two fields
// that resolve to the same delegate DocValuesFormat share a single
// underlying consumer and the same integer suffix (0), and that the
// FieldInfo attributes stamped on each field carry the delegate format
// name and that shared suffix.
func TestPerFieldDocValuesFormat_SuffixAssignment(t *testing.T) {
	format := newRecordingDocValuesFormat("Lucene90DocValuesFormat")

	fis := index.NewFieldInfos()
	for i, name := range []string{"dv1", "dv2"} {
		if err := fis.Add(newNumericFieldInfo(name, i)); err != nil {
			t.Fatalf("fis.Add(%q): %v", name, err)
		}
	}

	provider := FieldDocValuesFormatProviderFunc(func(string) DocValuesFormat { return format })
	pf := NewPerFieldDocValuesFormat(provider)

	ws, _, dir := newSegmentStates(t, fis)
	defer dir.Close()

	consumer, err := pf.FieldsConsumer(ws)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}

	for _, name := range []string{"dv1", "dv2"} {
		if err := consumer.AddNumericField(fis.GetByName(name), nopNumericIterator{}); err != nil {
			t.Fatalf("AddNumericField(%q): %v", name, err)
		}
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if got := len(format.consumers); got != 1 {
		t.Fatalf("delegate consumers: got %d, want 1", got)
	}
	if want := perFieldDocValuesSuffix("Lucene90DocValuesFormat", "0"); format.consumers[0].state.SegmentSuffix != want {
		t.Errorf("delegate SegmentSuffix: got %q, want %q",
			format.consumers[0].state.SegmentSuffix, want)
	}
	if got := format.consumers[0].addedNumeric; len(got) != 2 || got[0] != "dv1" || got[1] != "dv2" {
		t.Errorf("delegate.addedNumeric: got %v, want [dv1 dv2]", got)
	}

	for _, name := range []string{"dv1", "dv2"} {
		fi := fis.GetByName(name)
		if got := fi.GetAttribute(PER_FIELD_DOC_VALUES_FORMAT_KEY); got != "Lucene90DocValuesFormat" {
			t.Errorf("field %q: format attribute = %q", name, got)
		}
		if got := fi.GetAttribute(PER_FIELD_DOC_VALUES_SUFFIX_KEY); got != "0" {
			t.Errorf("field %q: suffix attribute = %q", name, got)
		}
	}
}

// TestPerFieldDocValuesFormat_DistinctFormatsBumpSuffix verifies that two
// fields resolving to different delegate DocValuesFormats produce two
// underlying consumers, each carrying its own "<formatName>_<n>" segment
// suffix, and that suffix counters are scoped per format-name.
func TestPerFieldDocValuesFormat_DistinctFormatsBumpSuffix(t *testing.T) {
	fast := newRecordingDocValuesFormat("FastDV")
	slow := newRecordingDocValuesFormat("SlowDV")

	fis := index.NewFieldInfos()
	for i, name := range []string{"dv1", "dv2", "dv3"} {
		if err := fis.Add(newNumericFieldInfo(name, i)); err != nil {
			t.Fatalf("fis.Add(%q): %v", name, err)
		}
	}

	provider := FieldDocValuesFormatProviderFunc(func(field string) DocValuesFormat {
		if field == "dv2" {
			return slow
		}
		return fast
	})
	pf := NewPerFieldDocValuesFormat(provider)

	ws, _, dir := newSegmentStates(t, fis)
	defer dir.Close()

	consumer, err := pf.FieldsConsumer(ws)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}
	for _, name := range []string{"dv1", "dv2", "dv3"} {
		if err := consumer.AddNumericField(fis.GetByName(name), nopNumericIterator{}); err != nil {
			t.Fatalf("AddNumericField(%q): %v", name, err)
		}
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if got := len(fast.consumers); got != 1 {
		t.Fatalf("FastDV consumers: got %d, want 1", got)
	}
	if got := len(slow.consumers); got != 1 {
		t.Fatalf("SlowDV consumers: got %d, want 1", got)
	}
	if want := perFieldDocValuesSuffix("FastDV", "0"); fast.consumers[0].state.SegmentSuffix != want {
		t.Errorf("FastDV SegmentSuffix: got %q, want %q",
			fast.consumers[0].state.SegmentSuffix, want)
	}
	if want := perFieldDocValuesSuffix("SlowDV", "0"); slow.consumers[0].state.SegmentSuffix != want {
		t.Errorf("SlowDV SegmentSuffix: got %q, want %q",
			slow.consumers[0].state.SegmentSuffix, want)
	}

	wantSuffix := map[string]struct {
		formatName, suffix string
	}{
		"dv1": {"FastDV", "0"},
		"dv2": {"SlowDV", "0"},
		"dv3": {"FastDV", "0"},
	}
	for name, want := range wantSuffix {
		fi := fis.GetByName(name)
		if got := fi.GetAttribute(PER_FIELD_DOC_VALUES_FORMAT_KEY); got != want.formatName {
			t.Errorf("field %q: format attribute = %q, want %q", name, got, want.formatName)
		}
		if got := fi.GetAttribute(PER_FIELD_DOC_VALUES_SUFFIX_KEY); got != want.suffix {
			t.Errorf("field %q: suffix attribute = %q, want %q", name, got, want.suffix)
		}
	}
}

// TestPerFieldDocValuesFormat_BumpSuffixPerFormatName verifies that two
// *different* DocValuesFormat instances that share the same name receive
// distinct consumers, with the suffix counter advancing from 0 to 1.
func TestPerFieldDocValuesFormat_BumpSuffixPerFormatName(t *testing.T) {
	const name = "Lucene90DocValuesFormat"
	first := newRecordingDocValuesFormat(name)
	second := newRecordingDocValuesFormat(name)

	fis := index.NewFieldInfos()
	for i, fname := range []string{"a", "b"} {
		if err := fis.Add(newNumericFieldInfo(fname, i)); err != nil {
			t.Fatalf("fis.Add(%q): %v", fname, err)
		}
	}

	provider := FieldDocValuesFormatProviderFunc(func(field string) DocValuesFormat {
		if field == "a" {
			return first
		}
		return second
	})
	pf := NewPerFieldDocValuesFormat(provider)

	ws, _, dir := newSegmentStates(t, fis)
	defer dir.Close()

	consumer, err := pf.FieldsConsumer(ws)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}
	for _, fname := range []string{"a", "b"} {
		if err := consumer.AddNumericField(fis.GetByName(fname), nopNumericIterator{}); err != nil {
			t.Fatalf("AddNumericField(%q): %v", fname, err)
		}
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if got := fis.GetByName("a").GetAttribute(PER_FIELD_DOC_VALUES_SUFFIX_KEY); got != "0" {
		t.Errorf("field a suffix = %q, want %q", got, "0")
	}
	if got := fis.GetByName("b").GetAttribute(PER_FIELD_DOC_VALUES_SUFFIX_KEY); got != "1" {
		t.Errorf("field b suffix = %q, want %q", got, "1")
	}
	if want := perFieldDocValuesSuffix(name, "0"); first.consumers[0].state.SegmentSuffix != want {
		t.Errorf("first SegmentSuffix: got %q, want %q",
			first.consumers[0].state.SegmentSuffix, want)
	}
	if want := perFieldDocValuesSuffix(name, "1"); second.consumers[0].state.SegmentSuffix != want {
		t.Errorf("second SegmentSuffix: got %q, want %q",
			second.consumers[0].state.SegmentSuffix, want)
	}
}

// TestPerFieldDocValuesFormat_ReaderDispatch verifies the reader resolves
// delegate producers via the new DocValuesFormatByName registry, opens one
// per "<formatName>_<n>" suffix, and routes per-field GetNumeric calls to
// the producer holding that field.
func TestPerFieldDocValuesFormat_ReaderDispatch(t *testing.T) {
	fast := newRecordingDocValuesFormat("FastDV")
	slow := newRecordingDocValuesFormat("SlowDV")

	fast.fieldsPerSuffix[perFieldDocValuesSuffix("FastDV", "0")] = []int{0, 2}
	slow.fieldsPerSuffix[perFieldDocValuesSuffix("SlowDV", "0")] = []int{1}

	RegisterDocValuesFormat(fast)
	RegisterDocValuesFormat(slow)
	t.Cleanup(func() {
		UnregisterDocValuesFormat("FastDV")
		UnregisterDocValuesFormat("SlowDV")
	})

	fis := index.NewFieldInfos()
	type entry struct {
		name, format, suffix string
	}
	for i, e := range []entry{
		{"a", "FastDV", "0"},
		{"b", "SlowDV", "0"},
		{"c", "FastDV", "0"},
	} {
		fi := newNumericFieldInfo(e.name, i)
		fi.PutCodecAttribute(PER_FIELD_DOC_VALUES_FORMAT_KEY, e.format)
		fi.PutCodecAttribute(PER_FIELD_DOC_VALUES_SUFFIX_KEY, e.suffix)
		if err := fis.Add(fi); err != nil {
			t.Fatalf("fis.Add(%q): %v", e.name, err)
		}
	}

	_, rs, dir := newSegmentStates(t, fis)
	defer dir.Close()

	pf := NewPerFieldDocValuesFormat(nil)
	producer, err := pf.FieldsProducer(rs)
	if err != nil {
		t.Fatalf("FieldsProducer: %v", err)
	}

	if got := len(fast.producers); got != 1 {
		t.Errorf("FastDV producers: got %d, want 1", got)
	}
	if got := len(slow.producers); got != 1 {
		t.Errorf("SlowDV producers: got %d, want 1", got)
	}

	for _, name := range []string{"a", "b", "c"} {
		fi := fis.GetByName(name)
		got, err := producer.GetNumeric(fi)
		if err != nil {
			t.Fatalf("GetNumeric(%q): %v", name, err)
		}
		if got == nil {
			t.Errorf("GetNumeric(%q) = nil, want non-nil", name)
		}
	}

	if err := producer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !fast.producers[0].closed {
		t.Error("FastDV producer was not closed")
	}
	if !slow.producers[0].closed {
		t.Error("SlowDV producer was not closed")
	}
}

// TestPerFieldDocValuesFormat_ReaderMissingSuffixAttribute confirms the
// reader rejects a FieldInfo carrying a format-name but no suffix
// attribute, matching the Java IllegalStateException.
func TestPerFieldDocValuesFormat_ReaderMissingSuffixAttribute(t *testing.T) {
	format := newRecordingDocValuesFormat("XDV")
	RegisterDocValuesFormat(format)
	t.Cleanup(func() { UnregisterDocValuesFormat("XDV") })

	fis := index.NewFieldInfos()
	fi := newNumericFieldInfo("a", 0)
	fi.PutCodecAttribute(PER_FIELD_DOC_VALUES_FORMAT_KEY, "XDV")
	// suffix attribute intentionally missing
	if err := fis.Add(fi); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	_, rs, dir := newSegmentStates(t, fis)
	defer dir.Close()

	pf := NewPerFieldDocValuesFormat(nil)
	if _, err := pf.FieldsProducer(rs); err == nil {
		t.Fatal("FieldsProducer succeeded; want error about missing suffix")
	}
}

// TestPerFieldDocValuesFormat_ReaderUnknownFormatName verifies that the
// reader surfaces a registry miss as an error rather than returning a
// half-built producer.
func TestPerFieldDocValuesFormat_ReaderUnknownFormatName(t *testing.T) {
	fis := index.NewFieldInfos()
	fi := newNumericFieldInfo("a", 0)
	fi.PutCodecAttribute(PER_FIELD_DOC_VALUES_FORMAT_KEY, "UnknownDV")
	fi.PutCodecAttribute(PER_FIELD_DOC_VALUES_SUFFIX_KEY, "0")
	if err := fis.Add(fi); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	_, rs, dir := newSegmentStates(t, fis)
	defer dir.Close()

	pf := NewPerFieldDocValuesFormat(nil)
	if _, err := pf.FieldsProducer(rs); err == nil {
		t.Fatal("FieldsProducer succeeded; want error about unknown format name")
	}
}

// TestPerFieldDocValuesFormat_NonDocValuesFieldsIgnored verifies that the
// reader skips FieldInfos that do not carry doc-values.
func TestPerFieldDocValuesFormat_NonDocValuesFieldsIgnored(t *testing.T) {
	format := newRecordingDocValuesFormat("DV1")
	RegisterDocValuesFormat(format)
	t.Cleanup(func() { UnregisterDocValuesFormat("DV1") })

	fis := index.NewFieldInfos()
	// Indexed-only field, no doc-values, no codec attributes.
	if err := fis.Add(index.NewFieldInfo("indexed_only", 0, index.FieldInfoOptions{
		IndexOptions: index.IndexOptionsDocs,
	})); err != nil {
		t.Fatalf("fis.Add(indexed_only): %v", err)
	}
	// Doc-values field with PerField metadata.
	dv := newNumericFieldInfo("dv", 1)
	dv.PutCodecAttribute(PER_FIELD_DOC_VALUES_FORMAT_KEY, "DV1")
	dv.PutCodecAttribute(PER_FIELD_DOC_VALUES_SUFFIX_KEY, "0")
	if err := fis.Add(dv); err != nil {
		t.Fatalf("fis.Add(dv): %v", err)
	}
	format.fieldsPerSuffix[perFieldDocValuesSuffix("DV1", "0")] = []int{1}

	_, rs, dir := newSegmentStates(t, fis)
	defer dir.Close()

	pf := NewPerFieldDocValuesFormat(nil)
	producer, err := pf.FieldsProducer(rs)
	if err != nil {
		t.Fatalf("FieldsProducer: %v", err)
	}
	defer producer.Close()

	// Non-doc-values field has no underlying producer; GetNumeric must
	// return (nil, nil) rather than dispatch.
	got, err := producer.GetNumeric(fis.GetByName("indexed_only"))
	if err != nil {
		t.Fatalf("GetNumeric(indexed_only): %v", err)
	}
	if got != nil {
		t.Errorf("GetNumeric(indexed_only) = %v, want nil", got)
	}
}

// TestPerFieldDocValuesFormat_RegistryRoundTrip is a focused sanity check
// on the new package-level DocValuesFormat registry.
func TestPerFieldDocValuesFormat_RegistryRoundTrip(t *testing.T) {
	const name = "RoundTripDV"
	format := newRecordingDocValuesFormat(name)

	if _, err := DocValuesFormatByName(name); err == nil {
		t.Fatal("DocValuesFormatByName succeeded before registration")
	}

	RegisterDocValuesFormat(format)
	got, err := DocValuesFormatByName(name)
	if err != nil {
		t.Fatalf("DocValuesFormatByName after register: %v", err)
	}
	if got != format {
		t.Errorf("DocValuesFormatByName returned %v, want the registered instance", got)
	}

	UnregisterDocValuesFormat(name)
	if _, err := DocValuesFormatByName(name); err == nil {
		t.Fatal("DocValuesFormatByName succeeded after unregister")
	}
}

// TestPerFieldDocValuesFormat_SuffixFormat documents the exact
// "<name>_<n>" shape that ends up on disk; the byte upgrade hinges on this.
func TestPerFieldDocValuesFormat_SuffixFormat(t *testing.T) {
	for _, tc := range []struct {
		formatName string
		suffix     int
		want       string
	}{
		{"Lucene90DocValuesFormat", 0, "Lucene90DocValuesFormat_0"},
		{"Lucene90DocValuesFormat", 1, "Lucene90DocValuesFormat_1"},
		{"X", 12, "X_12"},
	} {
		got := perFieldDocValuesSuffix(tc.formatName, strconv.Itoa(tc.suffix))
		if got != tc.want {
			t.Errorf("perFieldDocValuesSuffix(%q, %d) = %q, want %q",
				tc.formatName, tc.suffix, got, tc.want)
		}
	}
}

// TestPerFieldDocValuesFormat_FullSegmentSuffix verifies the DV-specific
// nesting behaviour: when the outer segment suffix is non-empty, it is
// preserved by joining with the inner per-format suffix using "_".
func TestPerFieldDocValuesFormat_FullSegmentSuffix(t *testing.T) {
	for _, tc := range []struct {
		outer, inner, want string
	}{
		{"", "Lucene90DocValuesFormat_0", "Lucene90DocValuesFormat_0"},
		{"outer", "Lucene90DocValuesFormat_0", "outer_Lucene90DocValuesFormat_0"},
	} {
		got := perFieldDocValuesFullSegmentSuffix(tc.outer, tc.inner)
		if got != tc.want {
			t.Errorf("perFieldDocValuesFullSegmentSuffix(%q,%q) = %q, want %q",
				tc.outer, tc.inner, got, tc.want)
		}
	}
}

// TestPerFieldDocValuesFormat_UpdatedFieldHonoursPriorSuffix verifies the
// doc-values-specific update path: when a field is being re-written
// (DocValuesGen != -1) and already carries a suffix attribute, the writer
// reuses that suffix instead of allocating a new one. This keeps updated
// generations co-located with the prior delegate file.
func TestPerFieldDocValuesFormat_UpdatedFieldHonoursPriorSuffix(t *testing.T) {
	format := newRecordingDocValuesFormat("UpdDV")

	fis := index.NewFieldInfos()
	updated := index.NewFieldInfo("a", 0, index.FieldInfoOptions{
		DocValuesType: index.DocValuesTypeNumeric,
		DocValuesGen:  3, // simulates an existing field being updated
	})
	updated.PutCodecAttribute(PER_FIELD_DOC_VALUES_FORMAT_KEY, "UpdDV")
	updated.PutCodecAttribute(PER_FIELD_DOC_VALUES_SUFFIX_KEY, "7")
	if err := fis.Add(updated); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	provider := FieldDocValuesFormatProviderFunc(func(string) DocValuesFormat { return format })
	pf := NewPerFieldDocValuesFormat(provider)

	ws, _, dir := newSegmentStates(t, fis)
	defer dir.Close()

	consumer, err := pf.FieldsConsumer(ws)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}
	if err := consumer.AddNumericField(updated, nopNumericIterator{}); err != nil {
		t.Fatalf("AddNumericField: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if got := updated.GetAttribute(PER_FIELD_DOC_VALUES_SUFFIX_KEY); got != "7" {
		t.Errorf("suffix attribute after update = %q, want %q", got, "7")
	}
	if want := perFieldDocValuesSuffix("UpdDV", "7"); format.consumers[0].state.SegmentSuffix != want {
		t.Errorf("delegate SegmentSuffix: got %q, want %q",
			format.consumers[0].state.SegmentSuffix, want)
	}
}

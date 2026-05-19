// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// perFieldMergeState ports the package-private utility
// org.apache.lucene.codecs.perfield.PerFieldMergeState from Apache Lucene
// 10.4.0. It restricts a merge view (MergeState plus per-segment
// FieldsProducers) to a chosen set of field names so each delegate format
// in a PerField* codec sees only the fields it owns.
//
// Lucene 10.4.0 deviation: upstream returns a fully-populated MergeState
// because Lucene's MergeState carries every per-segment reader slot
// (stored fields, term vectors, norms, doc-values, points, KNN vectors,
// info stream, executor, etc.). The current Gocene MergeState
// (index/merge_state.go) only models the fields the merge pipeline
// already wires (SegmentInfo, MergeFieldInfos, FieldInfos, MaxDocs,
// DocMaps, LiveDocs, Directory, NeedsIndexSort); the remaining slots
// arrive with backlog #2707. Until then, the FieldsProducers slice is
// passed explicitly alongside MergeState and the restricted view is
// returned as a (MergeState, []FieldsProducer) pair.

// RestrictFields returns a copy of in restricted to the given field
// names, together with restricted FieldsProducers wrapping each entry of
// fieldsProducers. The lengths of in.FieldInfos and fieldsProducers must
// match. A nil fieldsProducers entry stays nil in the output, matching
// Lucene's per-segment "no producer for this slot" contract.
//
// The returned MergeState shares the immutable per-segment slices of in
// (DocMaps, MaxDocs, LiveDocs, SegmentInfo, Directory, NeedsIndexSort).
// Only FieldInfos and MergeFieldInfos are rebuilt as filtered views.
func RestrictFields(
	in *index.MergeState,
	fieldsProducers []FieldsProducer,
	fields []string,
) (*index.MergeState, []FieldsProducer, error) {
	if in == nil {
		return nil, nil, fmt.Errorf("perfield: MergeState must not be nil")
	}
	if len(in.FieldInfos) != len(fieldsProducers) {
		return nil, nil, fmt.Errorf(
			"perfield: FieldInfos length (%d) does not match FieldsProducers length (%d)",
			len(in.FieldInfos), len(fieldsProducers),
		)
	}

	allow := newFieldAllowSet(fields)

	restrictedFieldInfos := make([]*index.FieldInfos, len(in.FieldInfos))
	for i, fi := range in.FieldInfos {
		restrictedFieldInfos[i] = filterFieldInfos(fi, allow)
	}

	restrictedProducers := make([]FieldsProducer, len(fieldsProducers))
	for i, fp := range fieldsProducers {
		if fp == nil {
			continue
		}
		restrictedProducers[i] = newFilterFieldsProducer(fp, allow)
	}

	restrictedMergeFI := filterFieldInfos(in.MergeFieldInfos, allow)

	out := &index.MergeState{
		SegmentInfo:     in.SegmentInfo,
		MergeFieldInfos: restrictedMergeFI,
		FieldInfos:      restrictedFieldInfos,
		MaxDocs:         in.MaxDocs,
		DocMaps:         in.DocMaps,
		LiveDocs:        in.LiveDocs,
		Directory:       in.Directory,
		NeedsIndexSort:  in.NeedsIndexSort,
	}
	return out, restrictedProducers, nil
}

// fieldAllowSet is the immutable, name-indexed allow list used by both
// the FieldInfos and FieldsProducer filters. It also caches an ordered
// snapshot of the input field names so error messages and iteration
// order remain stable, matching Lucene's HashSet/ArrayList split in
// PerFieldMergeState.
type fieldAllowSet struct {
	set     map[string]struct{}
	ordered []string
}

func newFieldAllowSet(fields []string) *fieldAllowSet {
	set := make(map[string]struct{}, len(fields))
	ordered := make([]string, 0, len(fields))
	for _, f := range fields {
		if _, dup := set[f]; dup {
			continue
		}
		set[f] = struct{}{}
		ordered = append(ordered, f)
	}
	return &fieldAllowSet{set: set, ordered: ordered}
}

func (a *fieldAllowSet) contains(name string) bool {
	if a == nil {
		return false
	}
	_, ok := a.set[name]
	return ok
}

// filterFieldInfos rebuilds a *index.FieldInfos containing only the
// FieldInfo entries whose name appears in allow. Because Gocene's
// FieldInfos is a concrete struct (no subclassing), the filtered view is
// produced by constructing a fresh FieldInfos and re-Adding the surviving
// FieldInfo objects. Aggregate booleans (HasNorms, HasTermVectors, ...)
// recompute correctly on the rebuilt set since they iterate byName.
//
// nil input yields nil output, matching the upstream pass-through when a
// per-segment FieldInfos slot is empty.
func filterFieldInfos(src *index.FieldInfos, allow *fieldAllowSet) *index.FieldInfos {
	if src == nil {
		return nil
	}
	out := index.NewFieldInfos()
	it := src.Iterator()
	for it.HasNext() {
		fi := it.Next()
		if fi == nil {
			continue
		}
		if !allow.contains(fi.Name()) {
			continue
		}
		// Field numbering must be preserved for codec compatibility.
		_ = out.Add(fi)
	}
	out.Freeze()
	return out
}

// filterFieldsProducer is the FieldsProducer wrapper that mirrors
// PerFieldMergeState.FilterFieldsProducer. It rejects Terms requests for
// fields outside the allow set with an IllegalArgumentException-equivalent
// error, and forwards Close to the wrapped producer.
type filterFieldsProducer struct {
	inner FieldsProducer
	allow *fieldAllowSet
}

func newFilterFieldsProducer(inner FieldsProducer, allow *fieldAllowSet) *filterFieldsProducer {
	return &filterFieldsProducer{inner: inner, allow: allow}
}

// Terms returns the terms enumeration for field. If field is not part of
// the restricted set the call returns an error rather than dispatching to
// the wrapped producer, matching upstream behaviour.
func (f *filterFieldsProducer) Terms(field string) (index.Terms, error) {
	if !f.allow.contains(field) {
		return nil, fmt.Errorf(
			"perfield: field %q is not accessible in the current merge context, available ones are: %v",
			field, f.allow.ordered,
		)
	}
	return f.inner.Terms(field)
}

// Close releases the wrapped producer. The filter holds no resources of
// its own beyond the allow set.
func (f *filterFieldsProducer) Close() error {
	return f.inner.Close()
}

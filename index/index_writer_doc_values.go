// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// pendingDocValuesUpdate captures a single UpdateDocValues or TryUpdateDocValue
// request buffered until the next Commit.
//
// Term-based updates (UpdateDocValues) populate term and field; the term
// selects documents at commit time.
//
// DocID-based updates (TryUpdateDocValue) populate segmentName and docID; the
// update is applied only to that leaf-local document when the segment name
// matches.
type pendingDocValuesUpdate struct {
	term        *Term
	segmentName string
	docID       int
	field       string
	value       interface{} // int64, []byte, or nil
}

// docValuesFieldUpdate holds the resolved per-document update for one field.
// A nil value means "reset to no value" (the document will not have a value
// for this field in the next generation).
type docValuesFieldUpdate struct {
	value interface{} // int64, []byte, or nil
}

// isReset reports whether this update removes the value from the document.
func (u docValuesFieldUpdate) isReset() bool {
	return u.value == nil
}

// numericValue returns the int64 value and true if this update is a numeric
// set operation. Returns false for resets or binary values.
func (u docValuesFieldUpdate) numericValue() (int64, bool) {
	if u.value == nil {
		return 0, false
	}
	v, ok := u.value.(int64)
	return v, ok
}

// binaryValue returns the []byte value and true if this update is a binary
// set operation. Returns false for resets or numeric values.
func (u docValuesFieldUpdate) binaryValue() ([]byte, bool) {
	if u.value == nil {
		return nil, false
	}
	v, ok := u.value.([]byte)
	return v, ok
}

// applyDocValuesUpdatesLocked applies all buffered UpdateDocValues requests to
// the committed segments listed in segs. It must be called with w.mu held.
//
// The implementation mirrors Lucene's ReadersAndUpdates.writeFieldUpdates:
// for each segment, resolve the update terms to local doc IDs, build per-field
// update maps, merge them with the existing on-disk DocValues, write a new
// generation of .dvd/.dvm and a new .fnm, and advance docValuesGen /
// fieldInfosGen on the affected SegmentCommitInfo.
func (w *IndexWriter) applyDocValuesUpdatesLocked(segs []*SegmentCommitInfo) error {
	if len(w.pendingDVUpdates) == 0 {
		return nil
	}

	codec := w.config.Codec()
	if codec == nil {
		return fmt.Errorf("commit: cannot apply doc-values updates: no codec configured")
	}
	if codec.DocValuesFormat() == nil {
		return fmt.Errorf("commit: cannot apply doc-values updates: codec %q has no DocValuesFormat", codec.Name())
	}

	// Make a defensive copy and clear the buffer so subsequent commit calls do
	// not re-apply the same updates.
	updates := w.pendingDVUpdates
	w.pendingDVUpdates = w.pendingDVUpdates[:0]

	// Determine which segments are touched by the pending updates. A segment is
	// touched when any updated field is known to the writer as a DocValues field,
	// even if that field did not exist in the segment originally.  In that case
	// the field is added to the segment's FieldInfos and the matching documents
	// receive the updated value.
	for _, sci := range segs {
		touched := false
		for _, u := range updates {
			if w.fieldDocValuesTypeLocked(u.field).HasDocValues() {
				touched = true
				break
			}
		}
		if !touched {
			continue
		}
		if err := w.applyDocValuesUpdatesForSegmentLocked(sci, updates, codec); err != nil {
			return fmt.Errorf("commit: apply doc-values updates to segment %s: %w", sci.Name(), err)
		}
	}

	return nil
}

// applyDocValuesUpdatesForSegmentLocked resolves term-based updates against one
// committed segment and writes a new generation of DV files when at least one
// matching document is found.
func (w *IndexWriter) applyDocValuesUpdatesForSegmentLocked(
	sci *SegmentCommitInfo,
	updates []pendingDocValuesUpdate,
	codec Codec,
) error {
	// Open a fully wired SegmentReader so term resolution and current DV reads
	// go through the codec (including CFS handling and per-generation overlays).
	sr, err := openSegmentReader(w.directory, sci)
	if err != nil {
		return fmt.Errorf("open segment reader: %w", err)
	}
	defer func() { _ = sr.Close() }()

	fieldUpdates := make(map[string]map[int]docValuesFieldUpdate)
	hasMatch := false
	for _, u := range updates {
		fi := sr.GetFieldInfos().GetByName(u.field)
		if fi == nil {
			// The field did not exist in this segment; look up the global
			// FieldInfo so the field can be added on update.
			fi = w.fieldInfoLocked(u.field)
		}
		if fi == nil || !fi.DocValuesType().HasDocValues() {
			continue
		}

		var docs []int
		var err error
		if u.term != nil {
			docs, err = resolveDocsForTerm(sr, u.term)
			if err != nil {
				return fmt.Errorf("resolve term %v: %w", u.term, err)
			}
		} else if u.segmentName == sci.Name() {
			// TryUpdateDocValue path: a single leaf-local document ID.
			if u.docID < 0 || u.docID >= sr.MaxDoc() {
				return fmt.Errorf("tryUpdateDocValue docID %d out of range for segment %s", u.docID, sci.Name())
			}
			docs = []int{u.docID}
		}
		if len(docs) == 0 {
			continue
		}
		hasMatch = true
		if fieldUpdates[u.field] == nil {
			fieldUpdates[u.field] = make(map[int]docValuesFieldUpdate, len(docs))
		}
		for _, doc := range docs {
			fieldUpdates[u.field][doc] = docValuesFieldUpdate{value: u.value}
		}
	}
	if !hasMatch {
		return nil
	}

	if err := w.writeMergedDocValues(sci, sr, fieldUpdates, codec); err != nil {
		return err
	}

	return nil
}

// resolveDocsForTerm returns the live local document IDs in sr that match the
// given term. Deleted documents are skipped so updates do not target removed
// docs.
func resolveDocsForTerm(sr *SegmentReader, term *Term) ([]int, error) {
	terms, err := sr.Terms(term.Field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}
	te, err := terms.GetIterator()
	if err != nil {
		return nil, err
	}
	if te == nil {
		return nil, nil
	}
	found, err := te.SeekExact(term)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	// Request the doc-ID stream only; this is all we need to enumerate matching
	// documents.
	postings, err := te.Postings(0)
	if err != nil {
		return nil, err
	}
	if postings == nil {
		return nil, nil
	}

	liveDocs := sr.GetLiveDocs()
	var docs []int
	for {
		doc, err := postings.NextDoc()
		if err != nil {
			return nil, err
		}
		if doc == NO_MORE_DOCS {
			break
		}
		if liveDocs != nil && !liveDocs.Get(doc) {
			continue
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

// writeMergedDocValues writes a new generation of DocValues and FieldInfos files
// for sci, merging the existing on-disk values with the supplied per-field
// updates.
func (w *IndexWriter) writeMergedDocValues(
	sci *SegmentCommitInfo,
	sr *SegmentReader,
	fieldUpdates map[string]map[int]docValuesFieldUpdate,
	codec Codec,
) error {
	segInfo := sci.SegmentInfo()
	currentInfos := sr.GetFieldInfos()

	// Compute the shared field-infos generation. Each updated field gets its own
	// doc-values generation so that every per-field .dvd/.dvm contains exactly one
	// field, matching Lucene's ReadersAndUpdates.writeFieldUpdates layout.
	newFIGen := int64(1)
	if sci.HasFieldInfosGen() {
		newFIGen = sci.FieldInfosGen() + 1
	}
	nextDVGen := int64(1)
	if sci.HasDocValuesGen() {
		nextDVGen = sci.DocValuesGen() + 1
	}

	// Assign a distinct doc-values generation to every updated field and build the
	// new in-memory/on-disk FieldInfos with per-field docValuesGen values.  Fields
	// that are being added to this segment (they exist globally but not in the
	// segment's current FieldInfos) are merged in from the writer's global view.
	fieldGens := make(map[string]int64, len(fieldUpdates))
	addedFields := make(map[string]*FieldInfo, len(fieldUpdates))
	for fieldName := range fieldUpdates {
		fieldGens[fieldName] = nextDVGen
		nextDVGen++
		if currentInfos.GetByName(fieldName) == nil {
			global := w.fieldInfoLocked(fieldName)
			if global == nil {
				return fmt.Errorf("field %q missing from segment %q and unknown globally", fieldName, segInfo.Name())
			}
			addedFields[fieldName] = global
		}
	}
	newInfos := cloneFieldInfosUpdatingDVGen(currentInfos, fieldGens, addedFields)

	// Read current values through the existing overlay so prior generations are
	// visible during the merge.
	dvp := sr.docValuesDelegate()

	// Snapshot the directory contents before writing so every file created by
	// this update generation can be registered for the deleter, even when the
	// per-field doc-values format writes files with a composite suffix that the
	// caller cannot predict.
	beforeFiles, err := w.directory.ListAll()
	if err != nil {
		return fmt.Errorf("list directory before writing DV update: %w", err)
	}
	beforeSet := make(map[string]struct{}, len(beforeFiles))
	for _, f := range beforeFiles {
		beforeSet[f] = struct{}{}
	}

	dvUpdateFiles := sci.DocValuesUpdatesFiles()

	// Write one .dvd/.dvm pair per updated field, each with its own generation.
	for fieldName, updates := range fieldUpdates {
		newFI := newInfos.GetByName(fieldName)
		if newFI == nil {
			return fmt.Errorf("field %q missing from cloned FieldInfos", fieldName)
		}
		// Read existing values through the *old* FieldInfo so the overlay
		// producer resolves the prior generation correctly.
		oldFI := currentInfos.GetByName(fieldName)
		gen := fieldGens[fieldName]
		suffix := strconv.FormatInt(gen, 36)

		dvInfos := NewFieldInfos()
		if err := dvInfos.Add(newFI); err != nil {
			return fmt.Errorf("add field %q to dv FieldInfos: %w", fieldName, err)
		}
		dvInfos.Freeze()

		writeState := &SegmentWriteState{
			Directory:     w.directory,
			SegmentInfo:   segInfo,
			FieldInfos:    dvInfos,
			SegmentSuffix: suffix,
		}
		consumer, err := codec.DocValuesFormat().FieldsConsumer(writeState)
		if err != nil {
			return fmt.Errorf("open doc-values consumer for %q: %w", fieldName, err)
		}

		writeErr := func() error {
			switch newFI.DocValuesType() {
			case DocValuesTypeNumeric:
				var old NumericDocValues
				if dvp != nil && oldFI != nil {
					old, err = dvp.GetNumeric(oldFI)
					if err != nil {
						return fmt.Errorf("read old numeric values for %q: %w", fieldName, err)
					}
				}
				it := newMergedNumericIterator(sr.MaxDoc(), old, updates)
				if err := consumer.AddNumericField(newFI, it); err != nil {
					return fmt.Errorf("write merged numeric field %q: %w", fieldName, err)
				}
			case DocValuesTypeBinary:
				var old BinaryDocValues
				if dvp != nil && oldFI != nil {
					old, err = dvp.GetBinary(oldFI)
					if err != nil {
						return fmt.Errorf("read old binary values for %q: %w", fieldName, err)
					}
				}
				it := newMergedBinaryIterator(sr.MaxDoc(), old, updates)
				if err := consumer.AddBinaryField(newFI, it); err != nil {
					return fmt.Errorf("write merged binary field %q: %w", fieldName, err)
				}
			default:
				return fmt.Errorf("unsupported doc-values type %v for field %q", newFI.DocValuesType(), fieldName)
			}
			return nil
		}()
		closeErr := consumer.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return fmt.Errorf("close doc-values consumer for %q: %w", fieldName, closeErr)
		}

	}

	// Write a single new .fnm containing all fields with their per-gen values.
	fiSuffix := strconv.FormatInt(newFIGen, 36)
	if err := codec.FieldInfosFormat().Write(w.directory, segInfo, fiSuffix, newInfos, store.IOContextWrite); err != nil {
		return fmt.Errorf("write updated FieldInfos: %w", err)
	}

	// Discover every file this update generation produced, including composite
	// names such as _N_G_<formatName>_<suffix>.{dvd,dvm} emitted by
	// PerFieldDocValuesFormat.
	afterFiles, err := w.directory.ListAll()
	if err != nil {
		return fmt.Errorf("list directory after writing DV update: %w", err)
	}
	segPrefix := segInfo.Name() + "_"
	var allNewFiles []string
	for _, f := range afterFiles {
		if _, ok := beforeSet[f]; ok {
			continue
		}
		if strings.HasPrefix(f, segPrefix) {
			allNewFiles = append(allNewFiles, f)
		}
	}
	if len(allNewFiles) == 0 {
		return fmt.Errorf("no files were written for DV update of segment %s", segInfo.Name())
	}

	// Map each new file to the field whose generation appears in its name.
	for fieldName, gen := range fieldGens {
		genStr := "_" + strconv.FormatInt(gen, 36) + "_"
		fi := newInfos.GetByName(fieldName)
		if fi == nil {
			continue
		}
		if dvUpdateFiles[fi.Number()] == nil {
			dvUpdateFiles[fi.Number()] = make(map[string]struct{})
		}
		for _, f := range allNewFiles {
			if strings.Contains(f, genStr) {
				dvUpdateFiles[fi.Number()][f] = struct{}{}
			}
		}
	}

	// Register the generation and updated FieldInfos in memory. The segment's
	// docValuesGen tracks the highest generation written.
	maxDVGen := nextDVGen - 1
	sci.SetDocValuesGen(maxDVGen)
	sci.SetFieldInfosGen(newFIGen)
	sci.SetInMemoryFieldInfos(newInfos)

	// Update file tracking for the deleter.
	existingFiles := segInfo.Files()
	existingFiles = append(existingFiles, allNewFiles...)
	segInfo.SetFiles(existingFiles)

	fieldInfosFiles := sci.FieldInfosFiles()
	for _, f := range allNewFiles {
		fieldInfosFiles[f] = struct{}{}
	}
	sci.SetFieldInfosFiles(fieldInfosFiles)

	sci.SetDocValuesUpdatesFiles(dvUpdateFiles)

	return nil
}

// cloneFieldInfosUpdatingDVGen returns a new FieldInfos containing every field
// in src plus any field in added that is not already present in src, with the
// docValuesGen of fields named in gens set to the per-field generation in gens.
func cloneFieldInfosUpdatingDVGen(
	src *FieldInfos,
	gens map[string]int64,
	added map[string]*FieldInfo,
) *FieldInfos {
	out := NewFieldInfos()
	seen := make(map[string]struct{})
	it := src.Iterator()
	for it.HasNext() {
		fi := it.Next()
		seen[fi.Name()] = struct{}{}
		opts := FieldInfoOptions{
			IndexOptions:             fi.IndexOptions(),
			DocValuesType:            fi.DocValuesType(),
			DocValuesSkipIndexType:   fi.DocValuesSkipIndexType(),
			DocValuesGen:             fi.DocValuesGen(),
			Stored:                   fi.IsStored(),
			Tokenized:                fi.IsTokenized(),
			OmitNorms:                fi.OmitNorms(),
			StoreTermVectors:         fi.StoreTermVectors(),
			StoreTermVectorPositions: fi.StoreTermVectorPositions(),
			StoreTermVectorOffsets:   fi.StoreTermVectorOffsets(),
			StoreTermVectorPayloads:  fi.StoreTermVectorPayloads(),
			PointDimensionCount:      fi.PointDimensionCount(),
			PointIndexDimensionCount: fi.PointIndexDimensionCount(),
			PointNumBytes:            fi.PointNumBytes(),
			VectorDimension:          fi.VectorDimension(),
			VectorEncoding:           fi.VectorEncoding(),
			VectorSimilarityFunction: fi.VectorSimilarityFunction(),
			IsSoftDeletesField:       fi.IsSoftDeletesField(),
			IsParentField:            fi.IsParentField(),
		}
		if gen, ok := gens[fi.Name()]; ok {
			opts.DocValuesGen = gen
		}
		clone := schema.NewFieldInfo(fi.Name(), fi.Number(), opts)
		for k, v := range fi.GetAttributes() {
			clone.PutCodecAttribute(k, v)
		}
		_ = out.Add(clone)
	}
	for name, fi := range added {
		if _, ok := seen[name]; ok {
			continue
		}
		opts := FieldInfoOptions{
			IndexOptions:             fi.IndexOptions(),
			DocValuesType:            fi.DocValuesType(),
			DocValuesSkipIndexType:   fi.DocValuesSkipIndexType(),
			DocValuesGen:             fi.DocValuesGen(),
			Stored:                   fi.IsStored(),
			Tokenized:                fi.IsTokenized(),
			OmitNorms:                fi.OmitNorms(),
			StoreTermVectors:         fi.StoreTermVectors(),
			StoreTermVectorPositions: fi.StoreTermVectorPositions(),
			StoreTermVectorOffsets:   fi.StoreTermVectorOffsets(),
			StoreTermVectorPayloads:  fi.StoreTermVectorPayloads(),
			PointDimensionCount:      fi.PointDimensionCount(),
			PointIndexDimensionCount: fi.PointIndexDimensionCount(),
			PointNumBytes:            fi.PointNumBytes(),
			VectorDimension:          fi.VectorDimension(),
			VectorEncoding:           fi.VectorEncoding(),
			VectorSimilarityFunction: fi.VectorSimilarityFunction(),
			IsSoftDeletesField:       fi.IsSoftDeletesField(),
			IsParentField:            fi.IsParentField(),
		}
		if gen, ok := gens[name]; ok {
			opts.DocValuesGen = gen
		}
		fieldNumber := fi.Number()
		if out.GetByNumber(fieldNumber) != nil {
			// Gocene assigns field numbers per-segment, so a field imported from
			// another segment can collide with an existing number in this
			// segment. Reassign to the next available local number so the added
			// field can coexist in this segment's FieldInfos.
			fieldNumber = out.GetNextFieldNumber()
		}
		clone := schema.NewFieldInfo(fi.Name(), fieldNumber, opts)
		for k, v := range fi.GetAttributes() {
			clone.PutCodecAttribute(k, v)
		}
		_ = out.Add(clone)
	}
	out.Freeze()
	return out
}

// mergedNumericIterator merges an existing NumericDocValues source with a set
// of per-document updates. Reset updates suppress the document entirely.
type mergedNumericIterator struct {
	maxDoc     int
	oldValues  NumericDocValues
	updates    map[int]docValuesFieldUpdate
	docIDs     []int // sorted update doc ids
	nextUpdate int
	docID      int
	value      int64
	oldDoc     int // cached old value doc, util.NO_MORE_DOCS when exhausted
}

// newMergedNumericIterator creates a writer-side iterator that overlays updates
// onto oldValues. The updates map is owned by the caller; the iterator reads it
// only.
func newMergedNumericIterator(maxDoc int, oldValues NumericDocValues, updates map[int]docValuesFieldUpdate) *mergedNumericIterator {
	docIDs := make([]int, 0, len(updates))
	for d := range updates {
		docIDs = append(docIDs, d)
	}
	sort.Ints(docIDs)

	it := &mergedNumericIterator{
		maxDoc:    maxDoc,
		oldValues: oldValues,
		updates:   updates,
		docIDs:    docIDs,
		oldDoc:    -1,
	}
	if oldValues != nil {
		// Prime the old-values stream. Errors are ignored here; the stream will
		// simply appear empty if reading fails.
		d, _ := oldValues.NextDoc()
		it.oldDoc = d
	} else {
		it.oldDoc = util.NO_MORE_DOCS
	}
	return it
}

// Next advances to the next document that has a merged value and reports
// whether one exists.
func (it *mergedNumericIterator) Next() bool {
	for {
		var updDoc int
		if it.nextUpdate < len(it.docIDs) {
			updDoc = it.docIDs[it.nextUpdate]
		} else {
			updDoc = util.NO_MORE_DOCS
		}

		oldDoc := it.oldDoc
		if oldDoc == util.NO_MORE_DOCS {
			oldDoc = math.MaxInt32
		}
		if updDoc == util.NO_MORE_DOCS {
			updDoc = math.MaxInt32
		}

		if oldDoc == math.MaxInt32 && updDoc == math.MaxInt32 {
			return false
		}

		switch {
		case updDoc < oldDoc:
			// Update-only document.
			upd := it.updates[updDoc]
			it.nextUpdate++
			if upd.isReset() {
				continue
			}
			v, ok := upd.numericValue()
			if !ok {
				continue
			}
			it.docID = updDoc
			it.value = v
			return true

		case updDoc == oldDoc:
			// Both streams have a value at the same document. The update wins;
			// a reset removes the document from the merged stream.
			upd := it.updates[updDoc]
			it.nextUpdate++
			// Advance oldValues past this document as well.
			d, _ := it.oldValues.NextDoc()
			it.oldDoc = d
			if upd.isReset() {
				continue
			}
			v, ok := upd.numericValue()
			if !ok {
				continue
			}
			it.docID = updDoc
			it.value = v
			return true

		default: // oldDoc < updDoc
			if oldDoc >= it.maxDoc {
				return false
			}
			v, err := it.oldValues.LongValue()
			if err != nil {
				return false
			}
			it.docID = oldDoc
			it.value = v
			d, _ := it.oldValues.NextDoc()
			it.oldDoc = d
			return true
		}
	}
}

// DocID returns the current document ID.
func (it *mergedNumericIterator) DocID() int { return it.docID }

// Value returns the current numeric value.
func (it *mergedNumericIterator) Value() int64 { return it.value }

// mergedBinaryIterator merges an existing BinaryDocValues source with a set of
// per-document updates. Reset updates suppress the document entirely.
type mergedBinaryIterator struct {
	maxDoc     int
	oldValues  BinaryDocValues
	updates    map[int]docValuesFieldUpdate
	docIDs     []int
	nextUpdate int
	docID      int
	value      []byte
	oldDoc     int
}

// newMergedBinaryIterator creates a writer-side iterator that overlays updates
// onto oldValues.
func newMergedBinaryIterator(maxDoc int, oldValues BinaryDocValues, updates map[int]docValuesFieldUpdate) *mergedBinaryIterator {
	docIDs := make([]int, 0, len(updates))
	for d := range updates {
		docIDs = append(docIDs, d)
	}
	sort.Ints(docIDs)

	it := &mergedBinaryIterator{
		maxDoc:    maxDoc,
		oldValues: oldValues,
		updates:   updates,
		docIDs:    docIDs,
		oldDoc:    -1,
	}
	if oldValues != nil {
		d, _ := oldValues.NextDoc()
		it.oldDoc = d
	} else {
		it.oldDoc = util.NO_MORE_DOCS
	}
	return it
}

// Next advances to the next document that has a merged binary value.
func (it *mergedBinaryIterator) Next() bool {
	for {
		var updDoc int
		if it.nextUpdate < len(it.docIDs) {
			updDoc = it.docIDs[it.nextUpdate]
		} else {
			updDoc = util.NO_MORE_DOCS
		}

		oldDoc := it.oldDoc
		if oldDoc == util.NO_MORE_DOCS {
			oldDoc = math.MaxInt32
		}
		if updDoc == util.NO_MORE_DOCS {
			updDoc = math.MaxInt32
		}

		if oldDoc == math.MaxInt32 && updDoc == math.MaxInt32 {
			return false
		}

		switch {
		case updDoc < oldDoc:
			upd := it.updates[updDoc]
			it.nextUpdate++
			if upd.isReset() {
				continue
			}
			v, ok := upd.binaryValue()
			if !ok {
				continue
			}
			it.docID = updDoc
			it.value = v
			return true

		case updDoc == oldDoc:
			upd := it.updates[updDoc]
			it.nextUpdate++
			d, _ := it.oldValues.NextDoc()
			it.oldDoc = d
			if upd.isReset() {
				continue
			}
			v, ok := upd.binaryValue()
			if !ok {
				continue
			}
			it.docID = updDoc
			it.value = v
			return true

		default:
			if oldDoc >= it.maxDoc {
				return false
			}
			v, err := it.oldValues.BinaryValue()
			if err != nil {
				return false
			}
			it.docID = oldDoc
			it.value = v
			d, _ := it.oldValues.NextDoc()
			it.oldDoc = d
			return true
		}
	}
}

// DocID returns the current document ID.
func (it *mergedBinaryIterator) DocID() int { return it.docID }

// Value returns the current binary value.
func (it *mergedBinaryIterator) Value() []byte { return it.value }

// TryUpdateDocValue updates a single document's DocValues column without
// re-indexing. The reader must be a LeafReader obtained from this writer (the
// document ID is local to that leaf). A nil value resets the field for that
// document. Returns the sequence number of the operation, or an error if the
// reader is not a leaf from this writer or the field does not have the right
// DocValues type.
//
// This mirrors Lucene's IndexWriter.tryUpdateDocValue(IndexReader, int, String,
// ...).
func (w *IndexWriter) TryUpdateDocValue(reader IndexReaderInterface, docID int, field string, value interface{}) (int64, error) {
	if err := w.ensureOpen(); err != nil {
		return -1, err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Reject index-sort fields, mirroring UpdateDocValues.
	if sort := w.config.IndexSort(); sort != nil {
		for _, sf := range sort.Fields() {
			if sf.Field() == field {
				return -1, fmt.Errorf(
					"cannot update doc values for field %q because it participates in the index sort",
					field)
			}
		}
	}

	sr, ok := reader.(*SegmentReader)
	if !ok {
		return -1, fmt.Errorf("reader must be a *SegmentReader from this writer")
	}
	if docID < 0 || docID >= sr.MaxDoc() {
		return -1, fmt.Errorf("docID %d out of range [0,%d)", docID, sr.MaxDoc())
	}

	fi := sr.GetFieldInfos().GetByName(field)
	if fi == nil || !fi.DocValuesType().HasDocValues() {
		return -1, fmt.Errorf("field %q has no doc values", field)
	}
	if err := validateDVValueType(fi, value); err != nil {
		return -1, err
	}

	sci := sr.GetSegmentCommitInfo()
	if sci == nil {
		return -1, fmt.Errorf("reader has no SegmentCommitInfo")
	}

	w.pendingDVUpdates = append(w.pendingDVUpdates, pendingDocValuesUpdate{
		segmentName: sci.Name(),
		docID:       docID,
		field:       field,
		value:       value,
	})

	return w.getNextSequenceNumber(), nil
}

// validateDVValueType checks that value is compatible with the field's
// DocValuesType. A nil value is allowed for any DocValues field and means
// "reset".
func validateDVValueType(fi *FieldInfo, value interface{}) error {
	if value == nil {
		return nil
	}
	switch fi.DocValuesType() {
	case DocValuesTypeNumeric:
		if _, ok := value.(int64); !ok {
			return fmt.Errorf("field %q has numeric doc values but value is %T", fi.Name(), value)
		}
	case DocValuesTypeBinary:
		if _, ok := value.([]byte); !ok {
			return fmt.Errorf("field %q has binary doc values but value is %T", fi.Name(), value)
		}
	default:
		return fmt.Errorf("field %q has unsupported doc values type %v for updates", fi.Name(), fi.DocValuesType())
	}
	return nil
}

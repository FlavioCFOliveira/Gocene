// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sharedterms

import (
	"cmp"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/uniformsplit"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// STUniformSplitTermsWriter extends uniformsplit.UniformSplitTermsWriter by
// sharing all the fields terms in the same dictionary and by writing all the
// fields of a term in the same block line.
//
// The TermsBlocksExtension block file contains all the term blocks for all
// fields. Each block line, for a single term, may have multiple fields
// index.TermState. The block file also contains the fields metadata at the end
// of the file.
//
// The TermsDictionaryExtension dictionary file contains a single trie (fst.FST
// bytes) for all fields.
//
// This structure is adapted when there are lots of fields. In this case the
// shared-terms dictionary trie is much smaller.
//
// This spi.FieldsConsumer requires a custom Merge method for efficiency. The
// regular merge would scan all the fields sequentially, which internally would
// scan the whole shared-terms dictionary as many times as there are fields.
// Whereas the custom merge directly scans the internal shared-terms dictionary
// of all segments to merge, thus scanning once whatever the number of fields
// is.
//
// Mirrors
// org.apache.lucene.codecs.uniformsplit.sharedterms.STUniformSplitTermsWriter
// from Apache Lucene 10.5.0, which extends
// org.apache.lucene.codecs.uniformsplit.UniformSplitTermsWriter.
type STUniformSplitTermsWriter struct {
	*uniformsplit.UniformSplitTermsWriter
}

// NewSTUniformSplitTermsWriter mirrors the public
// STUniformSplitTermsWriter(PostingsWriterBase, SegmentWriteState,
// BlockEncoder) constructor (STUniformSplitTermsWriter.java:80).
func NewSTUniformSplitTermsWriter(
	postingsWriter codecs.PostingsWriterBase,
	state *index.SegmentWriteState,
	blockEncoder uniformsplit.BlockEncoder,
) (*STUniformSplitTermsWriter, error) {
	return NewSTUniformSplitTermsWriterWithBlockSizes(
		postingsWriter,
		state,
		uniformsplit.DefaultTargetNumBlockLines,
		uniformsplit.DefaultDeltaNumLines,
		blockEncoder)
}

// NewSTUniformSplitTermsWriterWithBlockSizes mirrors the public
// STUniformSplitTermsWriter(PostingsWriterBase, SegmentWriteState, int, int,
// BlockEncoder) constructor (STUniformSplitTermsWriter.java:90). Go has no
// overloading, so the constructors are distinguished by name.
func NewSTUniformSplitTermsWriterWithBlockSizes(
	postingsWriter codecs.PostingsWriterBase,
	state *index.SegmentWriteState,
	targetNumBlockLines int,
	deltaNumLines int,
	blockEncoder uniformsplit.BlockEncoder,
) (*STUniformSplitTermsWriter, error) {
	return NewSTUniformSplitTermsWriterWithCodec(
		postingsWriter,
		state,
		targetNumBlockLines,
		deltaNumLines,
		blockEncoder,
		uniformsplit.FieldMetadataSerializerInstance,
		Name,
		VersionCurrent,
		TermsBlocksExtension,
		TermsDictionaryExtension)
}

// NewSTUniformSplitTermsWriterWithCodec mirrors the protected
// STUniformSplitTermsWriter constructor that takes the codec identity
// (STUniformSplitTermsWriter.java:108).
func NewSTUniformSplitTermsWriterWithCodec(
	postingsWriter codecs.PostingsWriterBase,
	state *index.SegmentWriteState,
	targetNumBlockLines int,
	deltaNumLines int,
	blockEncoder uniformsplit.BlockEncoder,
	fieldMetadataWriter *uniformsplit.FieldMetadataSerializer,
	codecName string,
	versionCurrent int32,
	termsBlocksExtension string,
	dictionaryExtension string,
) (*STUniformSplitTermsWriter, error) {
	base, err := uniformsplit.NewUniformSplitTermsWriterWithCodec(
		postingsWriter,
		state,
		targetNumBlockLines,
		deltaNumLines,
		blockEncoder,
		fieldMetadataWriter,
		codecName,
		versionCurrent,
		termsBlocksExtension,
		dictionaryExtension)
	if err != nil {
		return nil, err
	}
	w := &STUniformSplitTermsWriter{UniformSplitTermsWriter: base}
	// Java's `this` inside the inherited FieldsConsumer.merge is the
	// STUniformSplitTermsWriter being constructed, so the Write that
	// super.merge resolves is this class's override; see
	// codecs.FieldsConsumerBase.
	w.SetImpl(w)
	return w, nil
}

// Write mirrors STUniformSplitTermsWriter.write(Fields, NormsProducer)
// (STUniformSplitTermsWriter.java:134).
func (w *STUniformSplitTermsWriter) Write(fields spi.Fields, normsProducer codecs.NormsProducer) error {
	return w.writeSegment(
		func(blockWriter *STBlockWriter, dictionaryBuilder uniformsplit.IndexDictionaryBuilder) ([]*uniformsplit.FieldMetadata, error) {
			return w.writeSingleSegment(fields, normsProducer, blockWriter, dictionaryBuilder)
		})
}

// sharedTermsWriter renders the private functional interface
// STUniformSplitTermsWriter.SharedTermsWriter
// (STUniformSplitTermsWriter.java:420), whose single member is
// writeSharedTerms(STBlockWriter, IndexDictionary.Builder).
type sharedTermsWriter func(
	blockWriter *STBlockWriter,
	dictionaryBuilder uniformsplit.IndexDictionaryBuilder,
) ([]*uniformsplit.FieldMetadata, error)

// writeSegment writes the new segment with the provided sharedTermsWriter,
// which can be either a single segment writer, or a multiple segment merging
// writer.
//
// Mirrors the private STUniformSplitTermsWriter.writeSegment(SharedTermsWriter)
// (STUniformSplitTermsWriter.java:142).
func (w *STUniformSplitTermsWriter) writeSegment(termsWriter sharedTermsWriter) error {
	blockWriter := NewSTBlockWriter(
		w.BlockOutput, w.TargetNumBlockLines, w.DeltaNumLines, w.BlockEncoder)
	dictionaryBuilder, err := uniformsplit.NewFSTDictionaryBuilder()
	if err != nil {
		return err
	}
	fieldMetadataList, err := termsWriter(blockWriter, dictionaryBuilder)
	if err != nil {
		return err
	}
	if err := blockWriter.FinishLastBlock(dictionaryBuilder); err != nil {
		return err
	}
	fieldsNumber, err := w.writeFieldMetadataList(fieldMetadataList)
	if err != nil {
		return err
	}
	return w.WriteDictionary(fieldsNumber, dictionaryBuilder)
}

// writeSingleSegment mirrors the private
// STUniformSplitTermsWriter.writeSingleSegment
// (STUniformSplitTermsWriter.java:153).
func (w *STUniformSplitTermsWriter) writeSingleSegment(
	fields spi.Fields,
	normsProducer codecs.NormsProducer,
	blockWriter *STBlockWriter,
	dictionaryBuilder uniformsplit.IndexDictionaryBuilder,
) ([]*uniformsplit.FieldMetadata, error) {
	fieldsIter, err := newFieldsIterator(fields, w.FieldInfos)
	if err != nil {
		return nil, err
	}
	fieldMetadataList, err := w.createFieldMetadataList(fieldsIter, w.MaxDoc)
	if err != nil {
		return nil, err
	}
	fieldTermsQueue, err := w.createFieldTermsQueue(fields, fieldMetadataList)
	if err != nil {
		return nil, err
	}
	groupedFieldTerms := make([]fieldTermsOwner, 0, fieldTermsQueue.Size())
	termStates := make([]*FieldMetadataTermState, 0, fieldTermsQueue.Size())

	for fieldTermsQueue.Size() != 0 {
		topFieldTerms := fieldTermsQueue.popTerms()
		term := util.BytesRefDeepCopyOf(topFieldTerms.CurrentTerm())
		groupedFieldTerms = groupByTerm(fieldTermsQueue, topFieldTerms, groupedFieldTerms)
		termStates, err = w.writePostingLines(term, groupedFieldTerms, normsProducer, termStates)
		if err != nil {
			return nil, err
		}
		if err := blockWriter.AddLine(term, termStates, dictionaryBuilder); err != nil {
			return nil, err
		}
		if err := nextTermForIterators(groupedFieldTerms, fieldTermsQueue); err != nil {
			return nil, err
		}
	}
	return fieldMetadataList, nil
}

// createFieldMetadataList mirrors the private
// STUniformSplitTermsWriter.createFieldMetadataList(Iterator<FieldInfo>, int)
// (STUniformSplitTermsWriter.java:176).
func (w *STUniformSplitTermsWriter) createFieldMetadataList(
	fieldInfos spi.FieldInfosIterator,
	maxDoc int,
) ([]*uniformsplit.FieldMetadata, error) {
	var fieldMetadataList []*uniformsplit.FieldMetadata
	for fieldInfos.HasNext() {
		fieldMetadata, err := uniformsplit.NewFieldMetadata(fieldInfos.Next(), maxDoc)
		if err != nil {
			return nil, err
		}
		fieldMetadata.SetDictionaryStartFP(w.DictionaryOutput.GetFilePointer())
		fieldMetadataList = append(fieldMetadataList, fieldMetadata)
	}
	return fieldMetadataList, nil
}

// createFieldTermsQueue mirrors the private
// STUniformSplitTermsWriter.createFieldTermsQueue(Fields, List<FieldMetadata>)
// (STUniformSplitTermsWriter.java:186).
func (w *STUniformSplitTermsWriter) createFieldTermsQueue(
	fields spi.Fields,
	fieldMetadataList []*uniformsplit.FieldMetadata,
) (*termIteratorQueue[fieldTermsOwner], error) {
	fieldQueue, err := newTermIteratorQueue[fieldTermsOwner](len(fieldMetadataList))
	if err != nil {
		return nil, err
	}
	for _, fieldMetadata := range fieldMetadataList {
		terms, err := fields.Terms(fieldMetadata.GetFieldInfo().Name())
		if err != nil {
			return nil, err
		}
		if terms != nil {
			termsEnum, err := terms.Iterator()
			if err != nil {
				return nil, err
			}
			ft := newFieldTerms(fieldMetadata, termsEnum)
			hasTerm, err := ft.NextTerm()
			if err != nil {
				return nil, err
			}
			if hasTerm {
				// There is at least one term for the field.
				fieldQueue.Add(fieldTermsOwner(ft))
			}
		}
	}
	return fieldQueue, nil
}

// groupByTerm mirrors the private generic
// STUniformSplitTermsWriter.groupByTerm(TermIteratorQueue<T>,
// TermIterator<T>, List<TermIterator<T>>)
// (STUniformSplitTermsWriter.java:208). Java clears and refills the caller's
// list in place; Go returns the refilled slice, which the caller re-assigns,
// so the backing array is reused exactly as Java reuses the ArrayList.
func groupByTerm[T termIterator](
	termIteratorQueue *termIteratorQueue[T],
	topTermIterator T,
	groupedTermIterators []T,
) []T {
	groupedTermIterators = groupedTermIterators[:0]
	groupedTermIterators = append(groupedTermIterators, topTermIterator)
	for termIteratorQueue.Size() != 0 {
		it := termIteratorQueue.Top()
		if topTermIterator.CurrentTerm().BytesRefCompareTo(it.CurrentTerm()) != 0 {
			return groupedTermIterators
		}
		// Same term for another iterator. Combine the iterators.
		groupedTermIterators = append(groupedTermIterators, it)
		termIteratorQueue.Pop()
	}
	return groupedTermIterators
}

// writePostingLines mirrors the private
// STUniformSplitTermsWriter.writePostingLines(BytesRef, List<? extends
// TermIterator<FieldTerms>>, NormsProducer, List<FieldMetadataTermState>)
// (STUniformSplitTermsWriter.java:225). Java clears and refills the caller's
// termStates list in place; Go returns the refilled slice.
//
// PORT NOTE: Java calls postingsWriter.setField and discards its result, then
// calls writePostingLine, which re-reads the enum flags from the writer's own
// state. Gocene's codecs.PostingsWriterBase returns those flags from SetField
// instead of storing them, so they are carried into
// uniformsplit.UniformSplitTermsWriter.WritePostingLine as its last argument;
// see that method's own PORT NOTE.
func (w *STUniformSplitTermsWriter) writePostingLines(
	term *util.BytesRef,
	groupedFieldTerms []fieldTermsOwner,
	normsProducer codecs.NormsProducer,
	termStates []*FieldMetadataTermState,
) ([]*FieldMetadataTermState, error) {
	termStates = termStates[:0]
	for _, fieldTermIterator := range groupedFieldTerms {
		ft := fieldTermIterator.asFieldTerms()
		enumFlags, err := w.PostingsWriter.SetField(ft.fieldMetadata.GetFieldInfo())
		if err != nil {
			return nil, err
		}
		blockTermState, err := w.WritePostingLine(ft.termsEnum, ft.fieldMetadata, normsProducer, enumFlags)
		if err != nil {
			return nil, err
		}
		if blockTermState != nil {
			ft.fieldMetadata.SetLastTerm(term)
			termStates = append(termStates, NewFieldMetadataTermState(ft.fieldMetadata, blockTermState))
		}
	}
	return termStates, nil
}

// nextTermForIterators mirrors the private generic
// STUniformSplitTermsWriter.nextTermForIterators(List<? extends
// TermIterator<T>>, TermIteratorQueue<T>)
// (STUniformSplitTermsWriter.java:243).
func nextTermForIterators[T termIterator](
	termIterators []T,
	termIteratorQueue *termIteratorQueue[T],
) error {
	for _, it := range termIterators {
		hasTerm, err := it.NextTerm()
		if err != nil {
			return err
		}
		if hasTerm {
			// There is a next term for the iterator. Add it to the priority
			// queue.
			termIteratorQueue.Add(it)
		}
	}
	return nil
}

// writeFieldMetadataList mirrors the private
// STUniformSplitTermsWriter.writeFieldMetadataList(Collection<FieldMetadata>)
// (STUniformSplitTermsWriter.java:255).
func (w *STUniformSplitTermsWriter) writeFieldMetadataList(
	fieldMetadataList []*uniformsplit.FieldMetadata,
) (int32, error) {
	fieldsOutput := store.NewByteBuffersDataOutput()
	fieldsNumber := int32(0)
	for _, fieldMetadata := range fieldMetadataList {
		if fieldMetadata.GetNumTerms() > 0 {
			if err := w.FieldMetadataWriter.Write(fieldsOutput, fieldMetadata); err != nil {
				return 0, err
			}
			fieldsNumber++
		}
	}
	if err := w.WriteFieldsMetadata(fieldsNumber, fieldsOutput); err != nil {
		return 0, err
	}
	return fieldsNumber, nil
}

// WriteDictionary mirrors the protected
// STUniformSplitTermsWriter.writeDictionary(int, IndexDictionary.Builder)
// (STUniformSplitTermsWriter.java:268). Java overloads
// UniformSplitTermsWriter.writeDictionary; Go has no overloading, so this
// declaration shadows the inherited one, exactly as the Java overload hides
// nothing and is selected by its argument types. The inherited one-argument
// body is still reached, below, through the embedded base.
func (w *STUniformSplitTermsWriter) WriteDictionary(
	fieldsNumber int32,
	dictionaryBuilder uniformsplit.IndexDictionaryBuilder,
) error {
	if fieldsNumber > 0 {
		if err := w.UniformSplitTermsWriter.WriteDictionary(dictionaryBuilder); err != nil {
			return err
		}
	}
	return codecs.WriteFooter(w.DictionaryOutput)
}

// Merge mirrors STUniformSplitTermsWriter.merge(MergeState, NormsProducer)
// (STUniformSplitTermsWriter.java:276). It overrides the concrete
// FieldsConsumer.merge that uniformsplit.UniformSplitTermsWriter inherits, and
// falls back to it — Java's super.merge(...) — on the two paths Java does.
func (w *STUniformSplitTermsWriter) Merge(
	mergeState *index.MergeState,
	normsProducer codecs.NormsProducer,
) error {
	if mergeState.NeedsIndexSort {
		// This custom merging does not support sorted index.
		// Fall back to the default merge, which is inefficient for this
		// postings format.
		return w.FieldsConsumerBase.Merge(mergeState, normsProducer)
	}
	fieldsProducers := mergeState.FieldsProducers
	segmentTermsList := make([]termIterator, 0, len(fieldsProducers))
	for segmentIndex := 0; segmentIndex < len(fieldsProducers); segmentIndex++ {
		fieldsProducer := fieldsProducers[segmentIndex]
		// Iterate the FieldInfo provided by mergeState.FieldInfos because they
		// may be filtered by PerFieldMergeState.
		fieldInfoIter := mergeState.FieldInfos[segmentIndex].Iterator()
		for fieldInfoIter.HasNext() {
			fieldInfo := fieldInfoIter.Next()
			// Iterate all fields only to get the *first* Terms that is an
			// STUniformSplitTerms. See the break below.
			terms, err := fieldsProducer.Terms(fieldInfo.Name())
			if err != nil {
				return err
			}
			if terms != nil {
				sharedTerms, ok := terms.(*STUniformSplitTerms)
				if !ok {
					// Terms is not directly an instance of
					// STUniformSplitTerms, it is wrapped/filtered. Fall back
					// to the default merge, which is inefficient for this
					// postings format.
					return w.FieldsConsumerBase.Merge(mergeState, normsProducer)
				}
				mergingBlockReader, err := sharedTerms.createMergingBlockReader()
				if err != nil {
					return err
				}
				segmentTermsList = append(segmentTermsList, newSegmentTerms(
					segmentIndex, mergingBlockReader, mergeState.DocMaps[segmentIndex]))
				// We have the STUniformSplitTerms for the segment. Break the
				// field loop to iterate the next segment.
				break
			}
		}
	}
	return w.writeSegment(
		func(blockWriter *STBlockWriter, dictionaryBuilder uniformsplit.IndexDictionaryBuilder) ([]*uniformsplit.FieldMetadata, error) {
			return w.mergeSegments(mergeState, normsProducer, segmentTermsList, blockWriter, dictionaryBuilder)
		})
}

// mergeSegments mirrors the private
// STUniformSplitTermsWriter.mergeSegments(MergeState, NormsProducer,
// List<TermIterator<SegmentTerms>>, STBlockWriter, IndexDictionary.Builder)
// (STUniformSplitTermsWriter.java:317).
func (w *STUniformSplitTermsWriter) mergeSegments(
	mergeState *index.MergeState,
	normsProducer codecs.NormsProducer,
	segmentTermsList []termIterator,
	blockWriter *STBlockWriter,
	dictionaryBuilder uniformsplit.IndexDictionaryBuilder,
) ([]*uniformsplit.FieldMetadata, error) {
	fieldMetadataList, err := w.createFieldMetadataList(
		mergeState.MergeFieldInfos.Iterator(), mergeState.SegmentInfo.MaxDoc())
	if err != nil {
		return nil, err
	}
	fieldTermsMap := w.createMergingFieldTermsMap(fieldMetadataList, len(mergeState.FieldsProducers))
	segmentTermsQueue, err := w.createSegmentTermsQueue(segmentTermsList)
	if err != nil {
		return nil, err
	}
	groupedSegmentTerms := make([]termIterator, 0, len(segmentTermsList))
	fieldPostingsMap := make(map[string][]*segmentPostings, len(mergeState.FieldInfos))
	groupedFieldTerms := make([]fieldTermsOwner, 0, len(mergeState.FieldInfos))
	termStates := make([]*FieldMetadataTermState, 0, len(mergeState.FieldInfos))

	for segmentTermsQueue.Size() != 0 {
		topSegmentTerms := segmentTermsQueue.popTerms()
		term := util.BytesRefDeepCopyOf(topSegmentTerms.CurrentTerm())
		groupedSegmentTerms = groupByTerm(segmentTermsQueue, topSegmentTerms, groupedSegmentTerms)
		combineSegmentsFields(groupedSegmentTerms, fieldPostingsMap)
		groupedFieldTerms = combinePostingsPerField(term, fieldTermsMap, fieldPostingsMap, groupedFieldTerms)
		termStates, err = w.writePostingLines(term, groupedFieldTerms, normsProducer, termStates)
		if err != nil {
			return nil, err
		}
		if err := blockWriter.AddLine(term, termStates, dictionaryBuilder); err != nil {
			return nil, err
		}
		if err := nextTermForIterators(groupedSegmentTerms, segmentTermsQueue); err != nil {
			return nil, err
		}
	}
	return fieldMetadataList, nil
}

// createMergingFieldTermsMap mirrors the private
// STUniformSplitTermsWriter.createMergingFieldTermsMap(List<FieldMetadata>,
// int) (STUniformSplitTermsWriter.java:349).
func (w *STUniformSplitTermsWriter) createMergingFieldTermsMap(
	fieldMetadataList []*uniformsplit.FieldMetadata,
	numSegments int,
) map[string]*mergingFieldTerms {
	fieldTermsMap := make(map[string]*mergingFieldTerms, len(fieldMetadataList))
	for _, fieldMetadata := range fieldMetadataList {
		fieldInfo := fieldMetadata.GetFieldInfo()
		fieldTermsMap[fieldInfo.Name()] = newMergingFieldTerms(
			fieldMetadata, newSTMergingTermsEnum(fieldInfo.Name(), numSegments))
	}
	return fieldTermsMap
}

// createSegmentTermsQueue mirrors the private
// STUniformSplitTermsWriter.createSegmentTermsQueue(List<TermIterator<SegmentTerms>>)
// (STUniformSplitTermsWriter.java:362).
func (w *STUniformSplitTermsWriter) createSegmentTermsQueue(
	segmentTermsList []termIterator,
) (*termIteratorQueue[termIterator], error) {
	segmentQueue, err := newTermIteratorQueue[termIterator](len(segmentTermsList))
	if err != nil {
		return nil, err
	}
	for _, segmentTerms := range segmentTermsList {
		hasTerm, err := segmentTerms.NextTerm()
		if err != nil {
			return nil, err
		}
		if hasTerm {
			// There is at least one term in the segment.
			segmentQueue.Add(segmentTerms)
		}
	}
	return segmentQueue, nil
}

// combineSegmentsFields mirrors the private
// STUniformSplitTermsWriter.combineSegmentsFields(List<TermIterator<SegmentTerms>>,
// Map<String, List<SegmentPostings>>) (STUniformSplitTermsWriter.java:374).
func combineSegmentsFields(
	groupedSegmentTerms []termIterator,
	fieldPostingsMap map[string][]*segmentPostings,
) {
	clear(fieldPostingsMap)
	for _, segmentTermIterator := range groupedSegmentTerms {
		st := segmentTermIterator.(*segmentTerms)
		for field, termState := range st.fieldTermStatesMap {
			fieldPostingsMap[field] = append(fieldPostingsMap[field], newSegmentPostings(
				st.segmentIndex, termState, st.mergingBlockReader, st.docMap))
		}
	}
}

// combinePostingsPerField mirrors the private
// STUniformSplitTermsWriter.combinePostingsPerField(BytesRef, Map<String,
// MergingFieldTerms>, Map<String, List<SegmentPostings>>,
// List<MergingFieldTerms>) (STUniformSplitTermsWriter.java:396). Java clears
// and refills the caller's list in place; Go returns the refilled slice.
func combinePostingsPerField(
	term *util.BytesRef,
	fieldTermsMap map[string]*mergingFieldTerms,
	fieldPostingsMap map[string][]*segmentPostings,
	groupedFieldTerms []fieldTermsOwner,
) []fieldTermsOwner {
	groupedFieldTerms = groupedFieldTerms[:0]
	for field, segmentPostingsList := range fieldPostingsMap {
		// The field defined in fieldPostingsMap comes from the FieldInfos of
		// the SegmentReadState. The fieldTermsMap contains entries for fields
		// coming from the SegmentMergeSate. So it is possible that the field
		// is not present in fieldTermsMap because it is removed.
		fieldTerms, ok := fieldTermsMap[field]
		if ok {
			fieldTerms.resetIterator(term, segmentPostingsList)
			groupedFieldTerms = append(groupedFieldTerms, fieldTerms)
		}
	}
	// Keep the fields ordered by their number in the target merge segment.
	sort.Slice(groupedFieldTerms, func(i, j int) bool {
		return groupedFieldTerms[i].asFieldTerms().fieldMetadata.GetFieldInfo().Number() <
			groupedFieldTerms[j].asFieldTerms().fieldMetadata.GetFieldInfo().Number()
	})
	return groupedFieldTerms
}

// segmentPostings renders the package-private inner class
// STUniformSplitTermsWriter.SegmentPostings
// (STUniformSplitTermsWriter.java:425). Its body reads none of the enclosing
// instance, so the Go rendering carries no back-pointer.
type segmentPostings struct {
	segmentIndex       int
	termState          index.TermState
	mergingBlockReader *STMergingBlockReader
	docMap             index.DocMap
}

// newSegmentPostings mirrors the SegmentPostings(int, BlockTermState,
// STMergingBlockReader, MergeState.DocMap) constructor
// (STUniformSplitTermsWriter.java:432).
func newSegmentPostings(
	segmentIndex int,
	termState index.TermState,
	mergingBlockReader *STMergingBlockReader,
	docMap index.DocMap,
) *segmentPostings {
	return &segmentPostings{
		segmentIndex:       segmentIndex,
		termState:          termState,
		mergingBlockReader: mergingBlockReader,
		docMap:             docMap,
	}
}

// getPostings mirrors SegmentPostings.getPostings(String, PostingsEnum, int)
// (STUniformSplitTermsWriter.java:443).
func (s *segmentPostings) getPostings(
	fieldName string,
	reuse spi.PostingsEnum,
	flags int,
) (spi.PostingsEnum, error) {
	return s.mergingBlockReader.PostingsForField(fieldName, s.termState, reuse, flags)
}

// termIteratorQueue renders the private inner class
// STUniformSplitTermsWriter.TermIteratorQueue<T>
// (STUniformSplitTermsWriter.java:448), which extends
// util.PriorityQueue<TermIterator<T>> with lessThan(a, b) = a.compareTo(b) < 0.
type termIteratorQueue[T termIterator] struct {
	*util.PriorityQueue[T]
}

// newTermIteratorQueue mirrors the TermIteratorQueue(int) constructor
// (STUniformSplitTermsWriter.java:450).
func newTermIteratorQueue[T termIterator](numFields int) (*termIteratorQueue[T], error) {
	pq, err := util.NewPriorityQueue(numFields, func(a, b T) bool {
		return compareTermIterators(a, b) < 0
	})
	if err != nil {
		return nil, err
	}
	return &termIteratorQueue[T]{PriorityQueue: pq}, nil
}

// popTerms mirrors TermIteratorQueue.popTerms()
// (STUniformSplitTermsWriter.java:459), whose two assertions state that the
// popped iterator and its term are non-null.
func (q *termIteratorQueue[T]) popTerms() T {
	return q.Pop()
}

// termIterator renders the private abstract inner class
// STUniformSplitTermsWriter.TermIterator<T>
// (STUniformSplitTermsWriter.java:466). Java's type parameter only narrows the
// declared parameter of compareSecondary, whose implementations downcast
// anyway, so the Go rendering is non-generic and compareSecondary takes the
// interface. The Java field `term` is read by the enclosing class, so it is
// exposed here as CurrentTerm.
type termIterator interface {
	// CurrentTerm returns the value of the Java field TermIterator.term
	// (STUniformSplitTermsWriter.java:468).
	CurrentTerm() *util.BytesRef

	// NextTerm mirrors the abstract boolean nextTerm()
	// (STUniformSplitTermsWriter.java:470).
	NextTerm() (bool, error)

	// CompareSecondary mirrors the abstract int
	// compareSecondary(TermIterator<T>) (STUniformSplitTermsWriter.java:482).
	CompareSecondary(other termIterator) int
}

// compareTermIterators carries the body of
// TermIterator.compareTo(TermIterator<T>)
// (STUniformSplitTermsWriter.java:473), whose assertion states that the term
// is non-null, i.e. that an exhausted iterator is never compared.
func compareTermIterators(a, b termIterator) int {
	comparison := a.CurrentTerm().BytesRefCompareTo(b.CurrentTerm())
	if comparison == 0 {
		return a.CompareSecondary(b)
	}
	return comparison
}

// fieldTermsOwner renders the `List<? extends TermIterator<FieldTerms>>`
// parameter of writePostingLines, whose body downcasts each element to
// FieldTerms (STUniformSplitTermsWriter.java:232). Both fieldTerms and its
// subclass mergingFieldTerms satisfy it.
type fieldTermsOwner interface {
	termIterator

	// asFieldTerms renders Java's `(FieldTerms) fieldTermIterator` downcast.
	asFieldTerms() *fieldTerms
}

// fieldTerms renders the private inner class
// STUniformSplitTermsWriter.FieldTerms (STUniformSplitTermsWriter.java:486).
type fieldTerms struct {
	term *util.BytesRef

	fieldMetadata *uniformsplit.FieldMetadata
	termsEnum     spi.TermsEnum
}

// newFieldTerms mirrors the FieldTerms(FieldMetadata, TermsEnum) constructor
// (STUniformSplitTermsWriter.java:491).
func newFieldTerms(fieldMetadata *uniformsplit.FieldMetadata, termsEnum spi.TermsEnum) *fieldTerms {
	return &fieldTerms{fieldMetadata: fieldMetadata, termsEnum: termsEnum}
}

// CurrentTerm returns the Java field FieldTerms.term, inherited from
// TermIterator.
func (f *fieldTerms) CurrentTerm() *util.BytesRef { return f.term }

// asFieldTerms returns the receiver: it is already a FieldTerms.
func (f *fieldTerms) asFieldTerms() *fieldTerms { return f }

// NextTerm mirrors FieldTerms.nextTerm (STUniformSplitTermsWriter.java:497),
// whose body is `term = termsEnum.next(); return term != null`.
func (f *fieldTerms) NextTerm() (bool, error) {
	term, err := f.termsEnum.Next()
	if err != nil {
		return false, err
	}
	if term == nil {
		f.term = nil
		return false, nil
	}
	f.term = term.BytesValue()
	return f.term != nil, nil
}

// CompareSecondary mirrors FieldTerms.compareSecondary
// (STUniformSplitTermsWriter.java:503).
func (f *fieldTerms) CompareSecondary(other termIterator) int {
	return cmp.Compare(
		f.fieldMetadata.GetFieldInfo().Number(),
		other.(fieldTermsOwner).asFieldTerms().fieldMetadata.GetFieldInfo().Number())
}

// mergingFieldTerms renders the private inner class
// STUniformSplitTermsWriter.MergingFieldTerms
// (STUniformSplitTermsWriter.java:510), which extends FieldTerms.
type mergingFieldTerms struct {
	*fieldTerms
}

// newMergingFieldTerms mirrors the MergingFieldTerms(FieldMetadata,
// STMergingTermsEnum) constructor (STUniformSplitTermsWriter.java:512).
func newMergingFieldTerms(
	fieldMetadata *uniformsplit.FieldMetadata,
	termsEnum *stMergingTermsEnum,
) *mergingFieldTerms {
	return &mergingFieldTerms{fieldTerms: newFieldTerms(fieldMetadata, termsEnum)}
}

// resetIterator mirrors MergingFieldTerms.resetIterator(BytesRef,
// List<SegmentPostings>) (STUniformSplitTermsWriter.java:516), whose body
// downcasts termsEnum to STMergingTermsEnum and calls reset.
func (m *mergingFieldTerms) resetIterator(term *util.BytesRef, segmentPostingsList []*segmentPostings) {
	m.termsEnum.(*stMergingTermsEnum).reset(term, segmentPostingsList)
}

// segmentTerms renders the private inner class
// STUniformSplitTermsWriter.SegmentTerms (STUniformSplitTermsWriter.java:521).
type segmentTerms struct {
	term *util.BytesRef

	segmentIndex       int
	mergingBlockReader *STMergingBlockReader
	fieldTermStatesMap map[string]index.TermState
	docMap             index.DocMap
}

// newSegmentTerms mirrors the SegmentTerms(int, STMergingBlockReader,
// MergeState.DocMap) constructor (STUniformSplitTermsWriter.java:528).
func newSegmentTerms(
	segmentIndex int,
	mergingBlockReader *STMergingBlockReader,
	docMap index.DocMap,
) *segmentTerms {
	return &segmentTerms{
		segmentIndex:       segmentIndex,
		mergingBlockReader: mergingBlockReader,
		docMap:             docMap,
		fieldTermStatesMap: make(map[string]index.TermState),
	}
}

// CurrentTerm returns the Java field SegmentTerms.term, inherited from
// TermIterator.
func (s *segmentTerms) CurrentTerm() *util.BytesRef { return s.term }

// NextTerm mirrors SegmentTerms.nextTerm
// (STUniformSplitTermsWriter.java:536).
func (s *segmentTerms) NextTerm() (bool, error) {
	term, err := s.mergingBlockReader.Next()
	if err != nil {
		return false, err
	}
	if term == nil {
		s.term = nil
		return false, nil
	}
	s.term = term.BytesValue()
	if s.term == nil {
		return false, nil
	}
	if err := s.mergingBlockReader.ReadFieldTermStatesMap(s.fieldTermStatesMap); err != nil {
		return false, err
	}
	return true, nil
}

// CompareSecondary mirrors SegmentTerms.compareSecondary
// (STUniformSplitTermsWriter.java:545).
func (s *segmentTerms) CompareSecondary(other termIterator) int {
	return cmp.Compare(s.segmentIndex, other.(*segmentTerms).segmentIndex)
}

// fieldsIterator renders the private static class
// STUniformSplitTermsWriter.FieldsIterator
// (STUniformSplitTermsWriter.java:551), an Iterator<FieldInfo> over the field
// names a Fields exposes, resolved against a FieldInfos.
//
// PORT NOTE: Java's Fields.iterator() is a java.util.Iterator<String> whose
// hasNext() does not fail; Gocene's spi.Fields.Iterator() returns an error, so
// it is opened once by newFieldsIterator and the error surfaces there.
type fieldsIterator struct {
	fieldNames spi.FieldIterator
	fieldInfos *index.FieldInfos
}

// newFieldsIterator mirrors the FieldsIterator(Fields, FieldInfos)
// constructor (STUniformSplitTermsWriter.java:556).
func newFieldsIterator(fields spi.Fields, fieldInfos *index.FieldInfos) (*fieldsIterator, error) {
	fieldNames, err := fields.Iterator()
	if err != nil {
		return nil, err
	}
	return &fieldsIterator{fieldNames: fieldNames, fieldInfos: fieldInfos}, nil
}

// HasNext mirrors FieldsIterator.hasNext
// (STUniformSplitTermsWriter.java:562).
func (it *fieldsIterator) HasNext() bool { return it.fieldNames.HasNext() }

// Next mirrors FieldsIterator.next (STUniformSplitTermsWriter.java:567).
//
// PORT NOTE: Gocene's spi.FieldIterator.Next returns an error alongside the
// name; java.util.Iterator.next() cannot. The name is empty exactly when Java
// would have thrown NoSuchElementException, and FieldInfos.FieldInfo("")
// returns nil, so the two agree on producing no FieldInfo.
func (it *fieldsIterator) Next() *index.FieldInfo {
	name, err := it.fieldNames.Next()
	if err != nil || name == "" {
		return nil
	}
	return it.fieldInfos.FieldInfo(name)
}

var (
	_ spi.FieldsConsumer     = (*STUniformSplitTermsWriter)(nil)
	_ spi.FieldInfosIterator = (*fieldsIterator)(nil)
	_ fieldTermsOwner        = (*fieldTerms)(nil)
	_ fieldTermsOwner        = (*mergingFieldTerms)(nil)
	_ termIterator           = (*segmentTerms)(nil)
)

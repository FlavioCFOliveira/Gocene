package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// ordinalsTupleCursor is a local interface mirroring org.apache.lucene.document.column.OrdinalsTupleCursor.
type ordinalsTupleCursor interface {
	NextDoc() int
	OrdValue() int
}

// ordinalsCursor is a local interface mirroring org.apache.lucene.document.column.OrdinalsCursor.
type ordinalsCursor interface {
	Size() int
	NextOrd() int
}

// SortedDocValuesWriter buffers up pending byte[] per doc, deref and sorting via int ord, then flushes when segment
// flushes.
//
// This is the Go port of org.apache.lucene.index.SortedDocValuesWriter.
type SortedDocValuesWriter struct {
	hash           *util.BytesRefHash
	pending        *packed.PackedLongValuesBuilder
	docsWithField  *DocsWithFieldSet
	iwBytesUsed    *util.Counter
	bytesUsed      int64 // this currently only tracks differences in 'pending'
	fieldInfo      *FieldInfo
	lastDocID      int
	scratch       *SharedIndexingScratch

	finalOrds         *packed.PackedLongValues
	finalSortedValues []int
	finalOrdMap       []int
}

func NewSortedDocValuesWriter(fieldInfo *FieldInfo, iwBytesUsed *util.Counter, pool *util.ByteBlockPool, scratch *SharedIndexingScratch) *SortedDocValuesWriter {
	hash := util.NewBytesRefHashWithCapacity(
		pool,
		util.DefaultCapacity,
		util.NewDirectBytesStartArray(util.DefaultCapacity),
	)
	// Use delta-packed builder with a reasonable overhead ratio (e.g., 0.1 for COMPACT)
	pending, _ := packed.DeltaPackedBuilder(packed.PackedLongValuesDefaultPageSize, 0.1)
	docsWithField := NewDocsWithFieldSet()

	s := &SortedDocValuesWriter{
		fieldInfo:     fieldInfo,
		iwBytesUsed:   iwBytesUsed,
		scratch:       scratch,
		hash:          hash,
		pending:       pending,
		docsWithField: docsWithFieldSet,
		lastDocID:     -1,
	}

	s.bytesUsed = pending.RamBytesUsed()
	s.iwBytesUsed.AddAndGet(s.bytesUsed)
	return s
}

func (s *SortedDocValuesWriter) AddValue(docID int, value *util.BytesRef) error {
	if docID <= s.lastDocID {
		return fmt.Errorf("DocValuesField %q appears more than once in this document (only one value is allowed per field)", s.fieldInfo.Name)
	}
	if value == nil {
		return fmt.Errorf("field %q: null value not allowed", s.fieldInfo.Name)
	}
	if value.Length > (util.ByteBlockSize - 2) {
		return fmt.Errorf("DocValuesField %q is too large, must be <= %d", s.fieldInfo.Name, util.ByteBlockSize-2)
	}

	if err := s.addOneValue(value); err != nil {
		return err
	}
	if err := s.docsWithField.Add(docID); err != nil {
		return err
	}

	s.lastDocID = docID
	return nil
}

func (s *SortedDocValuesWriter) addOneValue(value *util.BytesRef) error {
	termID, err := s.hash.Add(value)
	if err != nil {
		return err
	}

	if termID < 0 {
		termID = -termID - 1
	} else {
		// reserve additional space for each unique value:
		// 1. when indexing, when hash is 50% full, rehash() suddenly needs 2*size ints.
		// 2. when flushing, we need 1 int per value (slot in the ordMap).
		s.iwBytesUsed.AddAndGet(2 * 4) // Integer.BYTES = 4
	}

	if err := s.pending.Add(int64(termID)); err != nil {
		return err
	}
	s.updateBytesUsed()
	return nil
}

func (s *SortedDocValuesWriter) AddOrdinalTuples(baseDocID int, dictionary []*util.BytesRef, cursor ordinalsTupleCursor) error {
	dictSize := len(dictionary)
	var ordToHash []int
	if dictSize <= SharedIndexingScratchIntsSize {
		ordToHash = s.scratch.IntsScratch()[:dictSize]
	} else {
		ordToHash = make([]int, dictSize)
	}

	for i := 0; i < dictSize; i++ {
		ordToHash[i] = -1
	}

	for {
		batchDocID := cursor.NextDoc()
		if batchDocID == spi.NoMoreDocs {
			break
		}
		docID := baseDocID + batchDocID
		if docID <= s.lastDocID {
			return fmt.Errorf("DocValuesField %q appears more than once in this document (only one value is allowed per field)", s.fieldInfo.Name)
		}
		ord := cursor.OrdValue()
		id, err := s.lookupOrTranslate(ord, dictionary, ordToHash)
		if err != nil {
			return err
		}
		if err := s.pending.Add(int64(id)); err != nil {
			return err
		}
		if err := s.docsWithField.Add(docID); err != nil {
			return err
		}
		s.lastDocID = docID
	}
	s.updateBytesUsed()
	return nil
}

func (s *SortedDocValuesWriter) AddDenseOrdinalValues(firstDocID int, dictionary []*util.BytesRef, cursor ordinalsCursor) error {
	n := cursor.Size()
	if n == 0 {
		return nil
	}

	dictSize := len(dictionary)
	var ordToHash []int
	if dictSize <= SharedIndexingScratchIntsSize {
		ordToHash = s.scratch.IntsScratch()[:dictSize]
	} else {
		ordToHash = make([]int, dictSize)
	}

	for i := 0; i < dictSize; i++ {
		ordToHash[i] = -1
	}

	processed := 0
	defer func() {
		if processed > 0 {
			for i := firstDocID; i < firstDocID+processed; i++ {
				_ = s.docsWithField.Add(i)
			}
			s.lastDocID = firstDocID + processed - 1
		}
		s.updateBytesUsed()
	}()

	for processed < n {
		ord := cursor.NextOrd()
		id, err := s.lookupOrTranslate(ord, dictionary, ordToHash)
		if err != nil {
			return err
		}
		if err := s.pending.Add(int64(id)); err != nil {
			return err
		}
		processed++
	}
	return nil
}

func (s *SortedDocValuesWriter) lookupOrTranslate(ord int, dictionary []*util.BytesRef, ordToHash []int) (int, error) {
	if ord < 0 || ord >= len(dictionary) {
		return 0, fmt.Errorf("DocValuesField %q: ordinal %d is out of range [0, %d)", s.fieldInfo.Name, ord, len(dictionary))
	}
	id := ordToHash[ord]
	if id < 0 {
		termID, err := s.hash.Add(dictionary[ord])
		if err != nil {
			return 0, err
		}
		id = termID
		if id < 0 {
			id = -id - 1
		} else {
			s.iwBytesUsed.AddAndGet(2 * 4)
		}
		ordToHash[ord] = id
	}
	return id, nil
}

func (s *SortedDocValuesWriter) updateBytesUsed() {
	newBytesUsed := s.pending.RamBytesUsed()
	s.iwBytesUsed.AddAndGet(newBytesUsed - s.bytesUsed)
	s.bytesUsed = newBytesUsed
}

func (s *SortedDocValuesWriter) finish() {
	if s.finalSortedValues == nil {
		valueCount := s.hash.Size()
		s.updateBytesUsed()
		s.finalSortedValues = s.hash.Sort()
		s.finalOrds = s.pending.Build()
		s.finalOrdMap = make([]int, valueCount)
		for ord := 0; ord < valueCount; ord++ {
			s.finalOrdMap[s.finalSortedValues[ord]] = ord
		}
	}
}

func (s *SortedDocValuesWriter) GetDocValues() spi.SortedDocValues {
	s.finish()
	return &bufferedSortedDocValues{
		hash:            s.hash,
		finalOrds:       s.finalOrds,
		sortedValues:    s.finalSortedValues,
		ordMap:          s.finalOrdMap,
		docsWithField:    s.docsWithField,
	}
}

func (s *SortedDocValuesWriter) Flush(state *spi.SegmentWriteState, sortMap spi.SorterDocMap, consumer spi.DocValuesConsumer) error {
	s.finish()

	producer := getDocValuesProducer(
		s.fieldInfo,
		s.hash,
		s.finalOrds,
		s.finalSortedValues,
		s.finalOrdMap,
		s.docsWithField,
		sortMap,
	)
	return consumer.AddSortedField(s.fieldInfo, producer)
}

func getDocValuesProducer(
	writerFieldInfo *FieldInfo,
	hash *util.BytesRefHash,
	ords *packed.PackedLongValues,
	sortedValues []int,
	ordMap []int,
	docsWithField *DocsWithFieldSet,
	sortMap spi.SorterDocMap,
) spi.DocValuesProducer {
	var sorted []int
	if sortMap != nil {
		sorted = sortDocValues(
			sortMap.Size(),
			sortMap,
			&bufferedSortedDocValues{
				hash:          hash,
				finalOrds:     ords,
				sortedValues:  sortedValues,
				ordMap:        ordMap,
				docsWithField: docsWithField,
			},
		)
	}

	return &sortedDocValuesProducer{
		writerFieldInfo: writerFieldInfo,
		hash:            hash,
		ords:            ords,
		sortedValues:    sortedValues,
		ordMap:          ordMap,
		docsWithField:   docsWithField,
		sorted:          sorted,
	}
}

func sortDocValues(maxDoc int, sortMap spi.SorterDocMap, oldValues spi.SortedDocValues) []int {
	ords := make([]int, maxDoc)
	for i := range ords {
		ords[i] = -1
	}
	for {
		docID, err := oldValues.NextDoc()
		if err != nil || docID == spi.NoMoreDocs {
			break
		}
		newDocID := sortMap.OldToNew(docID)
		ord, _ := oldValues.OrdValue()
		ords[newDocID] = ord
	}
	return ords
}

type sortedDocValuesProducer struct {
	writerFieldInfo *FieldInfo
	hash            *util.BytesRefHash
	ords            *packed.PackedLongValues
	sortedValues    []int
	ordMap          []int
	docsWithField   *DocsWithFieldSet
	sorted          []int
}

func (p *sortedDocValuesProducer) GetSorted(fieldInfoIn *FieldInfo) (spi.SortedDocValues, error) {
	if fieldInfoIn != p.writerFieldInfo {
		return nil, fmt.Errorf("wrong fieldInfo")
	}
	buf := &bufferedSortedDocValues{
		hash:          p.hash,
		finalOrds:     p.ords,
		sortedValues:  p.sortedValues,
		ordMap:        p.ordMap,
		docsWithField: p.docsWithField,
	}
	if p.sorted == nil {
		return buf, nil
	}
	return &sortingSortedDocValues{
		in:    buf,
		ords:  p.sorted,
		docID: -1,
	}, nil
}

type bufferedSortedDocValues struct {
	hash          *util.BytesRefHash
	scratch       *util.BytesRef
	sortedValues  []int
	ordMap        []int
	ord           int
	finalOrds     *packed.PackedLongValues
	docsWithField *DocsWithFieldSet
}

func (b *bufferedSortedDocValues) DocID() int {
	// In a real implementation, we'd need a DocIdSetIterator over docsWithField.
	return -1 // Placeholder
}

func (b *bufferedSortedDocValues) NextDoc() (int, error) {
	// In a real implementation, we'd iterate over docsWithField.
	return spi.NoMoreDocs, nil // Placeholder
}

func (b *bufferedSortedDocValues) Advance(target int) (int, error) {
	return spi.NoMoreDocs, nil // Placeholder
}

func (b *bufferedSortedDocValues) AdvanceExact(target int) (bool, error) {
	return b.docsWithField.Contains(target), nil
}

func (b *bufferedSortedDocValues) LongValue() (int64, error) {
	return int64(b.ord), nil
}

func (b *bufferedSortedDocValues) Cost() int64 {
	return 0 // Placeholder
}

func (b *bufferedSortedDocValues) OrdValue() (int, error) {
	return b.ord, nil
}

func (b *bufferedSortedDocValues) LookupOrd(ord int) ([]byte, error) {
	if b.scratch == nil {
		b.scratch = util.NewBytesRefEmpty()
	}
	b.hash.Get(b.sortedValues[ord], b.scratch)
	return b.scratch.ValidBytes(), nil
}

func (b *bufferedSortedDocValues) GetValueCount() int {
	return b.hash.Size()
}

type sortingSortedDocValues struct {
	in    spi.SortedDocValues
	ords  []int
	docID int
}

func (s *sortingSortedDocValues) DocID() int {
	return s.docID
}

func (s *sortingSortedDocValues) NextDoc() (int, error) {
	for {
		s.docID++
		if s.docID == len(s.ords) {
			s.docID = spi.NoMoreDocs
			break
		}
		if s.ords[s.docID] != -1 {
			break
		}
	}
	return s.docID, nil
}

func (s *sortingSortedDocValues) Advance(target int) (int, error) {
	return spi.NoMoreDocs, nil
}

func (s *sortingSortedDocValues) AdvanceExact(target int) (bool, error) {
	s.docID = target
	return s.ords[target] != -1, nil
}

func (s *sortingSortedDocValues) LongValue() (int64, error) {
	return int64(s.ords[s.docID]), nil
}

func (s *sortingSortedDocValues) OrdValue() (int, error) {
	return s.ords[s.docID], nil
}

func (s *sortingSortedDocValues) Cost() int64 {
	return s.in.Cost()
}

func (s *sortingSortedDocValues) LookupOrd(ord int) ([]byte, error) {
	return s.in.LookupOrd(ord)
}

func (s *sortingSortedDocValues) GetValueCount() int {
	return s.in.GetValueCount()
}

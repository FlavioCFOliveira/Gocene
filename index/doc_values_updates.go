// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"sync"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DocValuesUpdate represents a single document update for a specific field.
// This is used to update doc values without re-indexing the entire document.
type DocValuesUpdate struct {
	// Field name to update
	Field string
	// Term that identifies the document(s) to update
	Term *Term
	// Value to set (can be int64 for numeric or []byte for binary)
	Value interface{}
	// Type of doc values
	Type DocValuesType
}

// NumericDocValuesUpdate represents an update to a numeric doc values field.
type NumericDocValuesUpdate struct {
	DocValuesUpdate
	// Numeric value
	NumericValue int64
}

// BinaryDocValuesUpdate represents an update to a binary doc values field.
type BinaryDocValuesUpdate struct {
	DocValuesUpdate
	// Binary value
	BinaryValue []byte
}

// NewNumericDocValuesUpdate creates a new numeric doc values update.
func NewNumericDocValuesUpdate(term *Term, field string, value int64) *NumericDocValuesUpdate {
	return &NumericDocValuesUpdate{
		DocValuesUpdate: DocValuesUpdate{
			Field: field,
			Term:  term,
			Type:  DocValuesTypeNumeric,
		},
		NumericValue: value,
	}
}

// NewBinaryDocValuesUpdate creates a new binary doc values update.
func NewBinaryDocValuesUpdate(term *Term, field string, value []byte) *BinaryDocValuesUpdate {
	return &BinaryDocValuesUpdate{
		DocValuesUpdate: DocValuesUpdate{
			Field: field,
			Term:  term,
			Type:  DocValuesTypeBinary,
		},
		BinaryValue: value,
	}
}

// DocValuesUpdatePackage holds a package of updates for a specific field.
type DocValuesUpdatePackage struct {
	Field   string
	Updates map[string]*DocValuesUpdate // keyed by term text
	mu      sync.RWMutex
}

// NewDocValuesUpdatePackage creates a new update package for a field.
func NewDocValuesUpdatePackage(field string) *DocValuesUpdatePackage {
	return &DocValuesUpdatePackage{
		Field:   field,
		Updates: make(map[string]*DocValuesUpdate),
	}
}

// AddUpdate adds an update to the package.
func (p *DocValuesUpdatePackage) AddUpdate(update *DocValuesUpdate) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Updates[update.Term.Text()] = update
}

// GetUpdate retrieves an update by term text.
func (p *DocValuesUpdatePackage) GetUpdate(termText string) (*DocValuesUpdate, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	update, ok := p.Updates[termText]
	return update, ok
}

// GetAllUpdates returns all updates in the package.
func (p *DocValuesUpdatePackage) GetAllUpdates() []*DocValuesUpdate {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*DocValuesUpdate, 0, len(p.Updates))
	for _, update := range p.Updates {
		result = append(result, update)
	}
	return result
}

// Size returns the number of updates in the package.
func (p *DocValuesUpdatePackage) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.Updates)
}

// Clear removes all updates from the package.
func (p *DocValuesUpdatePackage) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Updates = make(map[string]*DocValuesUpdate)
}

// DocValuesUpdateQueue manages pending doc values updates.
type DocValuesUpdateQueue struct {
	packages map[string]*DocValuesUpdatePackage // keyed by field name
	mu       sync.RWMutex
}

// NewDocValuesUpdateQueue creates a new update queue.
func NewDocValuesUpdateQueue() *DocValuesUpdateQueue {
	return &DocValuesUpdateQueue{
		packages: make(map[string]*DocValuesUpdatePackage),
	}
}

// AddUpdate adds a doc values update to the queue.
func (q *DocValuesUpdateQueue) AddUpdate(update *DocValuesUpdate) {
	q.mu.Lock()
	defer q.mu.Unlock()

	pkg, ok := q.packages[update.Field]
	if !ok {
		pkg = NewDocValuesUpdatePackage(update.Field)
		q.packages[update.Field] = pkg
	}
	pkg.AddUpdate(update)
}

// GetPackage retrieves the update package for a field.
func (q *DocValuesUpdateQueue) GetPackage(field string) (*DocValuesUpdatePackage, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	pkg, ok := q.packages[field]
	return pkg, ok
}

// GetAllPackages returns all update packages.
func (q *DocValuesUpdateQueue) GetAllPackages() []*DocValuesUpdatePackage {
	q.mu.RLock()
	defer q.mu.RUnlock()
	result := make([]*DocValuesUpdatePackage, 0, len(q.packages))
	for _, pkg := range q.packages {
		result = append(result, pkg)
	}
	return result
}

// Size returns the total number of updates in the queue.
func (q *DocValuesUpdateQueue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	total := 0
	for _, pkg := range q.packages {
		total += pkg.Size()
	}
	return total
}

// Clear removes all updates from the queue.
func (q *DocValuesUpdateQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.packages = make(map[string]*DocValuesUpdatePackage)
}

// DocValuesUpdateWriter writes doc values updates to disk.
type DocValuesUpdateWriter struct {
	directory   store.Directory
	segmentInfo *SegmentInfo
}

// NewDocValuesUpdateWriter creates a new update writer.
func NewDocValuesUpdateWriter(directory store.Directory, segmentInfo *SegmentInfo) *DocValuesUpdateWriter {
	return &DocValuesUpdateWriter{
		directory:   directory,
		segmentInfo: segmentInfo,
	}
}

// WriteUpdates writes the pending updates to disk.
func (w *DocValuesUpdateWriter) WriteUpdates(queue *DocValuesUpdateQueue) error {
	if queue.Size() == 0 {
		return nil
	}

	// Create update file name
	updatesFileName := "_" + w.segmentInfo.Name() + ".dvu"

	// Open output stream
	out, err := w.directory.CreateOutput(updatesFileName, store.IOContextWrite)
	if err != nil {
		return err
	}
	defer out.Close()

	dataOut := store.NewByteBuffersDataOutput()

	// Write number of fields
	packages := queue.GetAllPackages()
	dataOut.WriteVInt(int32(len(packages)))

	// Write updates for each field
	for _, pkg := range packages {
		// Write field name
		dataOut.WriteString(pkg.Field)

		// Write number of updates for this field
		updates := pkg.GetAllUpdates()
		dataOut.WriteVInt(int32(len(updates)))

		// Write each update
		for _, update := range updates {
			// Write term text
			dataOut.WriteString(update.Term.Text())

			// Write value based on type
			switch update.Type {
			case DocValuesTypeNumeric:
				if nu, ok := update.Value.(*NumericDocValuesUpdate); ok {
					dataOut.WriteVLong(nu.NumericValue)
				}
			case DocValuesTypeBinary:
				if bu, ok := update.Value.(*BinaryDocValuesUpdate); ok {
					dataOut.WriteVInt(int32(len(bu.BinaryValue)))
					dataOut.WriteBytes(bu.BinaryValue)
				}
			}
		}
	}

	// Write to output using ToBufferList
	buffers := dataOut.ToWriteableBufferList()
	for _, buf := range buffers {
		out.WriteBytes(buf.Bytes())
	}

	return nil
}

// DocValuesUpdateReader reads doc values updates from disk.
type DocValuesUpdateReader struct {
	directory store.Directory
}

// NewDocValuesUpdateReader creates a new update reader.
func NewDocValuesUpdateReader(directory store.Directory) *DocValuesUpdateReader {
	return &DocValuesUpdateReader{
		directory: directory,
	}
}

// ReadUpdates reads updates from disk for a segment.
func (r *DocValuesUpdateReader) ReadUpdates(segmentInfo *SegmentInfo) (*DocValuesUpdateQueue, error) {
	queue := NewDocValuesUpdateQueue()

	updatesFileName := "_" + segmentInfo.Name() + ".dvu"

	// Check if updates file exists
	if !r.directory.FileExists(updatesFileName) {
		return queue, nil
	}

	// Open input stream
	in, err := r.directory.OpenInput(updatesFileName, store.IOContextRead)
	if err != nil {
		return nil, err
	}
	defer in.Close()

	// Read data into buffer
	length := in.Length()
	buf := make([]byte, length)
	err = in.ReadBytes(buf)
	if err != nil {
		return nil, err
	}

	dataIn := store.NewByteArrayDataInput(buf)

	// Read number of fields
	numFields, err := dataIn.ReadVInt()
	if err != nil {
		return nil, err
	}

	// Read updates for each field
	for i := int32(0); i < numFields; i++ {
		// Read field name
		field, err := dataIn.ReadString()
		if err != nil {
			return nil, err
		}

		// Read number of updates for this field
		numUpdates, err := dataIn.ReadVInt()
		if err != nil {
			return nil, err
		}

		// Read each update
		for j := int32(0); j < numUpdates; j++ {
			// Read term text
			termText, err := dataIn.ReadString()
			if err != nil {
				return nil, err
			}

			// Determine update type from field info
			// For now, assume numeric
			value, err := dataIn.ReadVLong()
			if err != nil {
				return nil, err
			}

			term := NewTerm(field, termText)
			update := &DocValuesUpdate{
				Field: field,
				Term:  term,
				Value: value,
				Type:  DocValuesTypeNumeric,
			}
			queue.AddUpdate(update)
		}
	}

	return queue, nil
}

// DocValuesUpdateMerger merges doc values updates during segment merge.
type DocValuesUpdateMerger struct {
	sourceReaders []*DocValuesUpdateReader
}

// NewDocValuesUpdateMerger creates a new update merger.
func NewDocValuesUpdateMerger(readers []*DocValuesUpdateReader) *DocValuesUpdateMerger {
	return &DocValuesUpdateMerger{
		sourceReaders: readers,
	}
}

// AddSourceReader adds a source reader for merging.
func (m *DocValuesUpdateMerger) AddSourceReader(reader *DocValuesUpdateReader) {
	m.sourceReaders = append(m.sourceReaders, reader)
}

// MergeUpdates merges updates from multiple source segments.
func (m *DocValuesUpdateMerger) MergeUpdates(segmentInfos *SegmentInfos) (*DocValuesUpdateQueue, error) {
	mergedQueue := NewDocValuesUpdateQueue()

	// Merge updates from all source segments
	for _, reader := range m.sourceReaders {
		for _, segmentCommitInfo := range segmentInfos.segments {
			segmentInfo := segmentCommitInfo.SegmentInfo()
			queue, err := reader.ReadUpdates(segmentInfo)
			if err != nil {
				return nil, err
			}

			// Remap doc IDs and add to merged queue
			for _, pkg := range queue.GetAllPackages() {
				for _, update := range pkg.GetAllUpdates() {
					// TODO: Remap doc ID based on merge state
					mergedQueue.AddUpdate(update)
				}
			}
		}
	}

	return mergedQueue, nil
}

// DocValuesUpdateApplication applies doc values updates to a segment.
type DocValuesUpdateApplication struct {
	reader     IndexReader
	writer     *DocValuesUpdateWriter
	commitLock sync.Mutex
}

// NewDocValuesUpdateApplication creates a new update application.
func NewDocValuesUpdateApplication(reader IndexReader, writer *DocValuesUpdateWriter) *DocValuesUpdateApplication {
	return &DocValuesUpdateApplication{
		reader: reader,
		writer: writer,
	}
}

// ApplyUpdates applies pending updates to the segment.
func (a *DocValuesUpdateApplication) ApplyUpdates(queue *DocValuesUpdateQueue) error {
	a.commitLock.Lock()
	defer a.commitLock.Unlock()

	if queue.Size() == 0 {
		return nil
	}

	// Write updates to disk
	err := a.writer.WriteUpdates(queue)
	if err != nil {
		return err
	}

	// Clear the queue after successful write
	queue.Clear()

	return nil
}

// BytesRefDocValuesIterator wraps a BinaryDocValues to provide BytesRef iteration.
type BytesRefDocValuesIterator struct {
	values  BinaryDocValues
	current *util.BytesRef
}

// NewBytesRefDocValuesIterator creates a new BytesRef iterator.
func NewBytesRefDocValuesIterator(values BinaryDocValues) *BytesRefDocValuesIterator {
	return &BytesRefDocValuesIterator{
		values:  values,
		current: util.NewBytesRefEmpty(),
	}
}

// NextDoc advances to the next document.
func (it *BytesRefDocValuesIterator) NextDoc() (int, error) {
	docID, err := it.values.NextDoc()
	if err != nil {
		return 0, err
	}
	return docID, nil
}

// DocID returns the current document ID.
func (it *BytesRefDocValuesIterator) DocID() int {
	return it.values.DocID()
}

// Advance advances to the specified document.
func (it *BytesRefDocValuesIterator) Advance(target int) (int, error) {
	docID, err := it.values.Advance(target)
	if err != nil {
		return 0, err
	}
	return docID, nil
}

// Get returns the binary value for the given document.
func (it *BytesRefDocValuesIterator) Get(docID int) (*util.BytesRef, error) {
	bytes, err := it.values.Get(docID)
	if err != nil {
		return nil, err
	}
	it.current = util.NewBytesRef(bytes)
	return it.current, nil
}

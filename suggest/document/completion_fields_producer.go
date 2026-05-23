package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// completionsTermsReaderEntry holds per-field FST offsets and weight range.
// Mirrors CompletionsTermsReader (the lightweight descriptor that is read from
// the .cmp index; the FST itself is loaded lazily).
type completionsTermsReaderEntry struct {
	offset    int64
	minWeight int64
	maxWeight int64
	fieldType byte
}

// RAMBytesUsed returns an estimate of RAM used by this entry.
func (e *completionsTermsReaderEntry) RAMBytesUsed() int64 {
	return 32 // fixed-size struct, no heap children
}

// CompletionFieldsProducer reads the completion index (.cmp) and dictionary
// (.lkp) files written by CompletionFieldsConsumer. The FST for each field is
// loaded lazily on first call to Terms(field).
//
// Mirrors org.apache.lucene.search.suggest.document.CompletionFieldsProducer.
type CompletionFieldsProducer struct {
	delegateFieldsProducer codecs.FieldsProducer
	readers                map[string]*completionsTermsReaderEntry
	dictIn                 store.IndexInput
}

// NewCompletionFieldsProducer opens the .cmp index and .lkp dictionary files
// for the given segment and reads the per-field offsets.
//
// Mirrors CompletionFieldsProducer(String, SegmentReadState).
func NewCompletionFieldsProducer(
	codecName string,
	state *codecs.SegmentReadState,
) (*CompletionFieldsProducer, error) {
	indexFile := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, completionIndexExtension)

	var delegateProducer codecs.FieldsProducer
	var dictIn store.IndexInput
	var success bool
	defer func() {
		if !success {
			if dictIn != nil {
				_ = dictIn.Close()
			}
			if delegateProducer != nil {
				_ = delegateProducer.Close()
			}
		}
	}()

	// open dict file (.lkp)
	dictFile := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, completionDictExtension)
	var err error
	dictIn, err = state.Directory.OpenInput(dictFile, store.IOContext{})
	if err != nil {
		return nil, fmt.Errorf("completion fields producer: open dict: %w", err)
	}

	if _, err := codecs.CheckIndexHeader(dictIn, codecName,
		completionCodecVersion, completionCodecVersion,
		state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return nil, fmt.Errorf("completion fields producer: check dict header: %w", err)
	}
	// validate footer without consuming the whole file (random access)
	if _, err := codecs.RetrieveChecksum(dictIn); err != nil {
		return nil, fmt.Errorf("completion fields producer: retrieve dict checksum: %w", err)
	}

	// open and read index file (.cmp)
	rawIndex, err := state.Directory.OpenInput(indexFile, store.IOContext{})
	if err != nil {
		return nil, fmt.Errorf("completion fields producer: open index: %w", err)
	}
	index_ := store.NewChecksumIndexInput(rawIndex)
	defer func() { _ = rawIndex.Close() }()

	if _, err := codecs.CheckIndexHeader(index_, codecName,
		completionCodecVersion, completionCodecVersion,
		state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return nil, fmt.Errorf("completion fields producer: check index header: %w", err)
	}

	// read delegate postings format name and create producer
	delegateName, err := index_.ReadString()
	if err != nil {
		return nil, fmt.Errorf("completion fields producer: read delegate name: %w", err)
	}
	delegateFmt, err := codecs.PostingsFormatByName(delegateName)
	if err != nil {
		return nil, fmt.Errorf("completion fields producer: unknown delegate postings format %q: %w", delegateName, err)
	}
	delegateProducer, err = delegateFmt.FieldsProducer(state)
	if err != nil {
		return nil, fmt.Errorf("completion fields producer: delegate fields producer: %w", err)
	}

	numFields, err := store.ReadVInt(index_)
	if err != nil {
		return nil, fmt.Errorf("completion fields producer: read num fields: %w", err)
	}
	readers := make(map[string]*completionsTermsReaderEntry, numFields)
	for i := int32(0); i < numFields; i++ {
		fieldNumber, err := store.ReadVInt(index_)
		if err != nil {
			return nil, err
		}
		offset, err := store.ReadVLong(index_)
		if err != nil {
			return nil, err
		}
		minWeight, err := store.ReadVLong(index_)
		if err != nil {
			return nil, err
		}
		maxWeight, err := store.ReadVLong(index_)
		if err != nil {
			return nil, err
		}
		fType, err := index_.ReadByte()
		if err != nil {
			return nil, err
		}
		fi := state.FieldInfos.GetByNumber(int(fieldNumber))
		if fi == nil {
			return nil, fmt.Errorf("completion fields producer: unknown field number %d", fieldNumber)
		}
		readers[fi.Name()] = &completionsTermsReaderEntry{
			offset:    offset,
			minWeight: minWeight,
			maxWeight: maxWeight,
			fieldType: fType,
		}
	}

	if _, err := codecs.CheckFooter(index_); err != nil {
		return nil, fmt.Errorf("completion fields producer: check index footer: %w", err)
	}

	success = true
	return &CompletionFieldsProducer{
		delegateFieldsProducer: delegateProducer,
		readers:                readers,
		dictIn:                 dictIn,
	}, nil
}

// newCompletionFieldsProducerForMerge creates a shallow copy used during
// segment merges. Mirrors CompletionFieldsProducer(FieldsProducer, Map).
func newCompletionFieldsProducerForMerge(
	delegate codecs.FieldsProducer,
	readers map[string]*completionsTermsReaderEntry,
) *CompletionFieldsProducer {
	return &CompletionFieldsProducer{
		delegateFieldsProducer: delegate,
		readers:                readers,
	}
}

// Close releases all open files and the delegate producer.
func (p *CompletionFieldsProducer) Close() error {
	var firstErr error
	if err := p.delegateFieldsProducer.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if p.dictIn != nil {
		if err := p.dictIn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// GetMergeInstance returns a lightweight copy suitable for use during merges.
// Mirrors CompletionFieldsProducer.getMergeInstance().
func (p *CompletionFieldsProducer) GetMergeInstance() codecs.FieldsProducer {
	return newCompletionFieldsProducerForMerge(p.delegateFieldsProducer, p.readers)
}

// RAMBytesUsed returns an estimate of heap memory consumed.
func (p *CompletionFieldsProducer) RAMBytesUsed() int64 {
	var total int64
	for _, r := range p.readers {
		total += r.RAMBytesUsed()
	}
	return total
}

// Terms returns the suggest-aware Terms for the named field. If the field has
// no completion data, the delegate's Terms are returned directly.
// Mirrors CompletionFieldsProducer.terms(String).
func (p *CompletionFieldsProducer) Terms(field string) (index.Terms, error) {
	delegateTerms, err := p.delegateFieldsProducer.Terms(field)
	if err != nil {
		return nil, err
	}
	if delegateTerms == nil {
		return nil, nil
	}
	entry, ok := p.readers[field]
	if !ok {
		return delegateTerms, nil
	}
	return newCompletionTermsView(delegateTerms, entry), nil
}

// Size returns the number of completion fields.
// Mirrors CompletionFieldsProducer.size().
func (p *CompletionFieldsProducer) Size() int {
	return len(p.readers)
}

var _ codecs.FieldsProducer = (*CompletionFieldsProducer)(nil)

// completionTermsView wraps a delegate Terms and exposes the completion
// metadata. Mirrors CompletionTerms.java (the terms wrapper returned by the
// producer).
type completionTermsView struct {
	index.Terms
	entry *completionsTermsReaderEntry
}

func newCompletionTermsView(inner index.Terms, entry *completionsTermsReaderEntry) *completionTermsView {
	return &completionTermsView{Terms: inner, entry: entry}
}

// MinWeight returns the minimum stored weight for this field.
func (c *completionTermsView) MinWeight() int64 { return c.entry.minWeight }

// MaxWeight returns the maximum stored weight for this field.
func (c *completionTermsView) MaxWeight() int64 { return c.entry.maxWeight }

// FieldType returns the byte type tag for this completion field.
func (c *completionTermsView) FieldType() byte { return c.entry.fieldType }

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Completion postings-format file extensions and codec constants.
// Mirrors the package-level constants in CompletionPostingsFormat.java.
const (
	completionDictExtension  = "lkp"
	completionIndexExtension = "cmp"
	completionCodecVersion   = int32(1)
)

// completionMetaData holds per-field FST metadata written to the .cmp index.
// Mirrors CompletionFieldsConsumer.CompletionMetaData.
type completionMetaData struct {
	filePointer int64
	minWeight   int64
	maxWeight   int64
	fieldType   byte
}

// CompletionFieldsConsumer writes per-field weighted FSTs to the Completion
// Dictionary (.lkp). On close it writes the per-field offsets and metadata to
// the Completion Index (.cmp).
//
// Mirrors org.apache.lucene.search.suggest.document.CompletionFieldsConsumer.
type CompletionFieldsConsumer struct {
	codecName                  string
	delegatePostingsFormatName string
	seenFields                 map[string]*completionMetaData
	state                      *codecs.SegmentWriteState
	dictOut                    store.IndexOutput
	delegateFieldsConsumer     codecs.FieldsConsumer
	closed                     bool
}

// NewCompletionFieldsConsumer creates the consumer. Opens the .lkp dict file
// and writes its header. Mirrors CompletionFieldsConsumer(String, PostingsFormat,
// SegmentWriteState).
func NewCompletionFieldsConsumer(
	codecName string,
	delegatePostingsFormat codecs.PostingsFormat,
	state *codecs.SegmentWriteState,
) (*CompletionFieldsConsumer, error) {
	c := &CompletionFieldsConsumer{
		codecName:                  codecName,
		delegatePostingsFormatName: delegatePostingsFormat.Name(),
		seenFields:                 make(map[string]*completionMetaData),
		state:                      state,
	}

	dictFile := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, completionDictExtension)
	var success bool
	defer func() {
		if !success {
			if c.dictOut != nil {
				_ = c.dictOut.Close()
			}
			if c.delegateFieldsConsumer != nil {
				_ = c.delegateFieldsConsumer.Close()
			}
		}
	}()

	delegateConsumer, err := delegatePostingsFormat.FieldsConsumer(state)
	if err != nil {
		return nil, fmt.Errorf("completion fields consumer: delegate fields consumer: %w", err)
	}
	c.delegateFieldsConsumer = delegateConsumer

	dictOut, err := state.Directory.CreateOutput(dictFile, store.IOContext{})
	if err != nil {
		return nil, fmt.Errorf("completion fields consumer: create dict file: %w", err)
	}
	c.dictOut = dictOut

	if err := codecs.WriteIndexHeader(dictOut, codecName, completionCodecVersion,
		state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return nil, fmt.Errorf("completion fields consumer: write dict header: %w", err)
	}

	success = true
	return c, nil
}

// Write processes all terms in the field, delegates to the wrapped consumer,
// and builds the completion FST. Mirrors
// CompletionFieldsConsumer.write(Fields, NormsProducer) — in Gocene the
// iteration is one field at a time.
func (c *CompletionFieldsConsumer) Write(field string, terms index.Terms) error {
	if err := c.delegateFieldsConsumer.Write(field, terms); err != nil {
		return err
	}

	tw := newCompletionTermWriter()
	termsEnum, err := terms.GetIterator()
	if err != nil {
		return err
	}

	for {
		t, err := termsEnum.Next()
		if err != nil {
			return err
		}
		if t == nil {
			break
		}
		if err := tw.writeTerm([]byte(t.Text()), termsEnum); err != nil {
			return err
		}
	}

	filePointer := c.dictOut.GetFilePointer()
	stored, err := tw.finish(c.dictOut)
	if err != nil {
		return err
	}
	if stored {
		c.seenFields[field] = &completionMetaData{
			filePointer: filePointer,
			minWeight:   tw.minWeight,
			maxWeight:   tw.maxWeight,
			fieldType:   tw.fieldType,
		}
	}
	return nil
}

// Close writes the .cmp index file (field numbers + FST offsets) and the
// dict file footer. Mirrors CompletionFieldsConsumer.close().
func (c *CompletionFieldsConsumer) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true

	indexFile := index.SegmentFileName(c.state.SegmentInfo.Name(), c.state.SegmentSuffix, completionIndexExtension)
	var success bool
	defer func() {
		if !success {
			if c.dictOut != nil {
				_ = c.dictOut.Close()
			}
			if c.delegateFieldsConsumer != nil {
				_ = c.delegateFieldsConsumer.Close()
			}
		}
	}()

	indexOut, err := c.state.Directory.CreateOutput(indexFile, store.IOContext{})
	if err != nil {
		return fmt.Errorf("completion fields consumer: create index file: %w", err)
	}
	defer func() {
		_ = indexOut.Close()
	}()

	if err := c.delegateFieldsConsumer.Close(); err != nil {
		return fmt.Errorf("completion fields consumer: close delegate: %w", err)
	}

	if err := codecs.WriteIndexHeader(indexOut, c.codecName, completionCodecVersion,
		c.state.SegmentInfo.GetID(), c.state.SegmentSuffix); err != nil {
		return fmt.Errorf("completion fields consumer: write index header: %w", err)
	}

	// write delegate postings format name
	if err := indexOut.WriteString(c.delegatePostingsFormatName); err != nil {
		return err
	}
	// write number of seen fields
	if err := store.WriteVInt(indexOut, int32(len(c.seenFields))); err != nil {
		return err
	}
	// write per-field entries
	for fieldName, meta := range c.seenFields {
		fi := c.state.FieldInfos.GetByName(fieldName)
		if fi == nil {
			return fmt.Errorf("completion fields consumer: unknown field %q", fieldName)
		}
		if err := store.WriteVInt(indexOut, int32(fi.Number())); err != nil {
			return err
		}
		if err := store.WriteVLong(indexOut, meta.filePointer); err != nil {
			return err
		}
		if err := store.WriteVLong(indexOut, meta.minWeight); err != nil {
			return err
		}
		if err := store.WriteVLong(indexOut, meta.maxWeight); err != nil {
			return err
		}
		if err := indexOut.WriteByte(meta.fieldType); err != nil {
			return err
		}
	}

	if err := codecs.WriteFooter(indexOut); err != nil {
		return fmt.Errorf("completion fields consumer: write index footer: %w", err)
	}
	if err := codecs.WriteFooter(c.dictOut); err != nil {
		return fmt.Errorf("completion fields consumer: write dict footer: %w", err)
	}
	if err := c.dictOut.Close(); err != nil {
		return err
	}

	success = true
	return nil
}

var _ codecs.FieldsConsumer = (*CompletionFieldsConsumer)(nil)

// completionTermWriter builds an NRT FST from term postings, accumulating
// weight statistics. Mirrors CompletionFieldsConsumer.CompletionTermWriter.
type completionTermWriter struct {
	docCount  int
	maxWeight int64
	minWeight int64
	fieldType byte
	first     bool
	scratch   []byte
	builder   *NRTSuggesterBuilder
}

func newCompletionTermWriter() *completionTermWriter {
	builder, _ := NewNRTSuggesterBuilder()
	return &completionTermWriter{
		minWeight: int64(^uint64(0) >> 1), // Long.MAX_VALUE
		first:     true,
		builder:   builder,
	}
}

// finish writes the FST to output and returns true if anything was stored.
// Mirrors CompletionTermWriter.finish(IndexOutput).
func (tw *completionTermWriter) finish(output store.IndexOutput) (bool, error) {
	stored, err := tw.builder.Store(output)
	if err != nil {
		return false, err
	}
	if !stored && tw.docCount != 0 {
		return false, fmt.Errorf("completion term writer: FST is nil but docCount=%d", tw.docCount)
	}
	if tw.docCount == 0 {
		tw.minWeight = 0
	}
	return stored, nil
}

// writeTerm processes postings for a single term. Mirrors
// CompletionTermWriter.write(BytesRef, TermsEnum).
func (tw *completionTermWriter) writeTerm(term []byte, termsEnum index.TermsEnum) error {
	postingsEnum, err := termsEnum.Postings(postingsFlagPayloads)
	if err != nil {
		return err
	}
	if postingsEnum == nil {
		return nil
	}

	tw.builder.StartTerm(term)
	docFreq := 0

	for {
		docID, err := postingsEnum.NextDoc()
		if err != nil {
			return err
		}
		if docID == index.NO_MORE_DOCS {
			break
		}

		freq, err := postingsEnum.Freq()
		if err != nil {
			return err
		}
		for i := 0; i < freq; i++ {
			if _, err := postingsEnum.NextPosition(); err != nil {
				return err
			}
			payload, err := postingsEnum.GetPayload()
			if err != nil {
				return err
			}
			if payload == nil {
				return fmt.Errorf("completion term writer: payload is nil")
			}

			// payload format: vint(len) + bytes(surfaceForm) + vint(weight+1) + byte(type)
			in := newByteArrayDataInput(payload)
			surfaceLen, err := in.readVInt()
			if err != nil {
				return err
			}
			tw.scratch = make([]byte, surfaceLen)
			if err := in.readBytes(tw.scratch); err != nil {
				return err
			}
			weight, err := in.readVInt()
			if err != nil {
				return err
			}
			w := int64(weight) - 1
			if w > tw.maxWeight {
				tw.maxWeight = w
			}
			if w < tw.minWeight {
				tw.minWeight = w
			}
			fType, err := in.readByte()
			if err != nil {
				return err
			}
			if tw.first {
				tw.fieldType = fType
				tw.first = false
			} else if tw.fieldType != fType {
				return fmt.Errorf("completion term writer: single field name has mixed types")
			}
			if err := tw.builder.AddEntry(docID, tw.scratch, w); err != nil {
				return err
			}
		}
		docFreq++
		if docFreq+1 > tw.docCount {
			tw.docCount = docFreq + 1
		}
	}

	tw.builder.FinishTerm() //nolint:errcheck // FinishTerm errors are returned by Store
	return nil
}

// byteArrayDataInput is a minimal reader for payload bytes.
type byteArrayDataInput struct {
	data []byte
	pos  int
}

func newByteArrayDataInput(b []byte) *byteArrayDataInput {
	return &byteArrayDataInput{data: b}
}

func (r *byteArrayDataInput) readByte() (byte, error) {
	if r.pos >= len(r.data) {
		return 0, fmt.Errorf("byte array data input: EOF")
	}
	b := r.data[r.pos]
	r.pos++
	return b, nil
}

func (r *byteArrayDataInput) readVInt() (int32, error) {
	var b byte
	var v int32
	for shift := 0; ; shift += 7 {
		var err error
		b, err = r.readByte()
		if err != nil {
			return 0, err
		}
		v |= int32(b&0x7F) << shift
		if b&0x80 == 0 {
			break
		}
	}
	return v, nil
}

func (r *byteArrayDataInput) readBytes(dst []byte) error {
	if r.pos+len(dst) > len(r.data) {
		return fmt.Errorf("byte array data input: EOF reading %d bytes", len(dst))
	}
	copy(dst, r.data[r.pos:r.pos+len(dst)])
	r.pos += len(dst)
	return nil
}

// postingsFlagPayloads requests payload information from PostingsEnum.
// Mirrors org.apache.lucene.index.PostingsEnum.PAYLOADS (positions | 1<<6 = 24).
// Per Gocene freq_prox_fields.go: FREQS=1<<3, POSITIONS=FREQS|1<<4,
// PAYLOADS=POSITIONS|1<<6.
const postingsFlagPayloads = (8 | 16 | 64) // FREQS|POSITIONS|PAYLOADS = 88

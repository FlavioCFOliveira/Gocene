// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"fmt"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Postings flags — mirrors the package-private postingsFlag* constants in the
// index package (index/freq_prox_fields.go). Kept local to avoid coupling to
// unexported index internals.
const (
	pfFreqs     = 1 << 3         // freq
	pfPositions = pfFreqs | 1<<4 // positions (implies freqs)
	pfOffsets   = pfPositions | 1<<5
	pfPayloads  = pfPositions | 1<<6
)

// SimpleTextFieldsWriter writes postings as plain text for debugging.
//
// Port of org.apache.lucene.codecs.simpletext.SimpleTextFieldsWriter
// (Lucene 10.4.0).
//
// Deviation from Java: Go's FieldsConsumer.Write receives one field at a
// time (no NormsProducer). Norms are assumed to be 1 for all documents;
// skip-list impacts use freq only. The skip list structure is otherwise
// identical.
type SimpleTextFieldsWriter struct {
	// out is wrapped in ChecksumIndexOutput so that stWriteChecksum can
	// query the running CRC32 at close time.
	out        *store.ChecksumIndexOutput
	scratch    *util.BytesRefBuilder
	writeState *codecs.SegmentWriteState

	// docCount is the running document count for the current term.
	docCount int

	skipWriter *SimpleTextSkipWriter

	accumulator        *codecs.CompetitiveImpactAccumulator
	lastDocFilePointer int64

	closed bool
}

// NewSimpleTextFieldsWriter opens the postings file and prepares the writer.
//
// Port of SimpleTextFieldsWriter(SegmentWriteState).
func NewSimpleTextFieldsWriter(state *codecs.SegmentWriteState) (*SimpleTextFieldsWriter, error) {
	fileName := index.SegmentFileName(
		state.SegmentInfo.Name(),
		state.SegmentSuffix,
		postingsExtension,
	)
	raw, err := state.Directory.CreateOutput(fileName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return nil, fmt.Errorf("SimpleTextFieldsWriter: create %s: %w", fileName, err)
	}
	return &SimpleTextFieldsWriter{
		out:         store.NewChecksumIndexOutput(raw),
		scratch:     util.NewBytesRefBuilder(),
		writeState:  state,
		skipWriter:  NewSimpleTextSkipWriter(state.SegmentInfo.DocCount()),
		accumulator: codecs.NewCompetitiveImpactAccumulator(),
	}, nil
}

// Write encodes all postings for the given field.
//
// Port of SimpleTextFieldsWriter.write(Fields, NormsProducer) adapted to the
// Go FieldsConsumer.Write(field string, terms index.Terms) contract.
func (w *SimpleTextFieldsWriter) Write(field string, terms index.Terms) error {
	if w.closed {
		return fmt.Errorf("SimpleTextFieldsWriter: already closed")
	}

	fi := w.writeState.FieldInfos.GetByName(field)
	// fi may be nil if the field has no FieldInfo registered; treat norms as
	// absent in that case.
	var fieldHasNorms bool
	var hasPayloads bool
	if fi != nil {
		fieldHasNorms = fi.HasNorms()
		hasPayloads = fi.HasPayloads()
	}

	hasPositions := terms.HasPositions()
	hasFreqs := terms.HasFreqs()
	hasOffsets := terms.HasOffsets()

	// Compute postings flags.
	flags := 0
	if hasPositions {
		flags = pfPositions
		if hasPayloads {
			flags |= pfPayloads
		}
		if hasOffsets {
			flags |= pfOffsets
		}
	} else if hasFreqs {
		flags |= pfFreqs
	}

	termsEnum, err := terms.GetIterator()
	if err != nil {
		return fmt.Errorf("SimpleTextFieldsWriter.Write(%q): GetIterator: %w", field, err)
	}

	wroteField := false
	var postingsEnum index.PostingsEnum

	// For each term in the field.
	for {
		term, err := termsEnum.Next()
		if err != nil {
			return fmt.Errorf("SimpleTextFieldsWriter.Write(%q): Next: %w", field, err)
		}
		if term == nil {
			break
		}

		w.docCount = 0
		w.skipWriter.resetSkip()
		w.accumulator.Clear()
		w.lastDocFilePointer = -1

		postingsEnum, err = termsEnum.Postings(flags)
		if err != nil {
			return fmt.Errorf("SimpleTextFieldsWriter.Write(%q): Postings: %w", field, err)
		}

		wroteTerm := false

		// For each doc in field+term.
		for {
			doc, err := postingsEnum.NextDoc()
			if err != nil {
				return fmt.Errorf("SimpleTextFieldsWriter.Write(%q): NextDoc: %w", field, err)
			}
			if doc == index.NO_MORE_DOCS {
				break
			}

			if !wroteTerm {
				if !wroteField {
					if err := w.writeBytes(stfwField); err != nil {
						return err
					}
					if err := w.writeStr(field); err != nil {
						return err
					}
					if err := w.newline(); err != nil {
						return err
					}
					wroteField = true
				}
				if err := w.writeBytes(stfwTerm); err != nil {
					return err
				}
				// Write the raw term bytes (escape-processed) to match what
				// SimpleTextFieldsReader decodes.
				termBytes := term.BytesValue()
				if termBytes != nil {
					if err := w.writeBytes(termBytes.Bytes[:termBytes.Length]); err != nil {
						return err
					}
				}
				if err := w.newline(); err != nil {
					return err
				}
				wroteTerm = true
			}

			if w.lastDocFilePointer == -1 {
				w.lastDocFilePointer = w.out.GetFilePointer()
			}

			if err := w.writeBytes(stfwDoc); err != nil {
				return err
			}
			if err := w.writeStr(strconv.Itoa(doc)); err != nil {
				return err
			}
			if err := w.newline(); err != nil {
				return err
			}

			if hasFreqs {
				freq, err := postingsEnum.Freq()
				if err != nil {
					return fmt.Errorf("SimpleTextFieldsWriter.Write(%q): Freq: %w", field, err)
				}
				if err := w.writeBytes(stfwFreq); err != nil {
					return err
				}
				if err := w.writeStr(strconv.Itoa(freq)); err != nil {
					return err
				}
				if err := w.newline(); err != nil {
					return err
				}

				if hasPositions {
					for i := 0; i < freq; i++ {
						pos, err := postingsEnum.NextPosition()
						if err != nil {
							return fmt.Errorf("SimpleTextFieldsWriter.Write(%q): NextPosition: %w", field, err)
						}
						if err := w.writeBytes(stfwPos); err != nil {
							return err
						}
						if err := w.writeStr(strconv.Itoa(pos)); err != nil {
							return err
						}
						if err := w.newline(); err != nil {
							return err
						}

						if hasOffsets {
							startOff, err := postingsEnum.StartOffset()
							if err != nil {
								return fmt.Errorf("SimpleTextFieldsWriter.Write(%q): StartOffset: %w", field, err)
							}
							endOff, err := postingsEnum.EndOffset()
							if err != nil {
								return fmt.Errorf("SimpleTextFieldsWriter.Write(%q): EndOffset: %w", field, err)
							}
							if err := w.writeBytes(stfwStartOffset); err != nil {
								return err
							}
							if err := w.writeStr(strconv.Itoa(startOff)); err != nil {
								return err
							}
							if err := w.newline(); err != nil {
								return err
							}
							if err := w.writeBytes(stfwEndOffset); err != nil {
								return err
							}
							if err := w.writeStr(strconv.Itoa(endOff)); err != nil {
								return err
							}
							if err := w.newline(); err != nil {
								return err
							}
						}

						payload, err := postingsEnum.GetPayload()
						if err != nil {
							return fmt.Errorf("SimpleTextFieldsWriter.Write(%q): GetPayload: %w", field, err)
						}
						if len(payload) > 0 {
							if err := w.writeBytes(stfwPayload); err != nil {
								return err
							}
							if err := w.writeBytes(payload); err != nil {
								return err
							}
							if err := w.newline(); err != nil {
								return err
							}
						}
					}
				}

				norm := w.getNorm(fieldHasNorms)
				w.accumulator.Add(freq, norm)
			} else {
				norm := w.getNorm(fieldHasNorms)
				w.accumulator.Add(1, norm)
			}

			w.docCount++
			if w.docCount != 0 && w.docCount%skipBlockSize == 0 {
				if err := w.skipWriter.bufferSkip(
					doc,
					w.lastDocFilePointer,
					w.docCount,
					w.accumulator,
				); err != nil {
					return fmt.Errorf("SimpleTextFieldsWriter.Write(%q): bufferSkip: %w", field, err)
				}
				w.accumulator.Clear()
				w.lastDocFilePointer = -1
			}
		}

		if w.docCount >= skipBlockSize {
			if _, err := w.skipWriter.WriteSkip(w.out); err != nil {
				return fmt.Errorf("SimpleTextFieldsWriter.Write(%q): WriteSkip: %w", field, err)
			}
		}
	}

	return nil
}

// getNorm returns the norm value for a document. Since Go's FieldsConsumer
// does not receive a NormsProducer, norms are not available here and the
// default value 1 is returned, matching Java's getNorm when norms == null.
func (w *SimpleTextFieldsWriter) getNorm(fieldHasNorms bool) int64 {
	// Deviation from Java: NormsProducer is not accessible through Go's
	// FieldsConsumer.Write API. Always return 1 (the identity norm).
	return 1
}

// Close writes the END marker and checksum, then closes the output.
//
// Port of SimpleTextFieldsWriter.close().
func (w *SimpleTextFieldsWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	if w.out == nil {
		return nil
	}
	out := w.out
	w.out = nil

	var closeErr error
	defer func() {
		if cerr := out.Close(); cerr != nil && closeErr == nil {
			closeErr = fmt.Errorf("SimpleTextFieldsWriter.Close: %w", cerr)
		}
	}()

	if err := w.writeBytes(stfwEnd); err != nil {
		closeErr = err
		return closeErr
	}
	if err := w.newline(); err != nil {
		closeErr = err
		return closeErr
	}
	if err := stWriteChecksum(out, w.scratch); err != nil {
		closeErr = err
		return closeErr
	}
	return closeErr
}

// ---------------------------------------------------------------------------
// Output helpers
// ---------------------------------------------------------------------------

func (w *SimpleTextFieldsWriter) writeBytes(b []byte) error {
	return stWrite(w.out, b, w.scratch)
}

func (w *SimpleTextFieldsWriter) writeStr(s string) error {
	return stWriteStr(w.out, s, w.scratch)
}

func (w *SimpleTextFieldsWriter) newline() error {
	return stWriteNewline(w.out)
}

// compile-time assertion.
var _ codecs.FieldsConsumer = (*SimpleTextFieldsWriter)(nil)

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// PushPostingsWriterBase extends the PostingsWriterBase contract with a
// "push" API that lets the caller drive the posting-list writer
// position-by-position. It is the Go port of
// org.apache.lucene.codecs.PushPostingsWriterBase from Apache Lucene 10.4.0.
//
// The Java original is an abstract class; in Go we model it as an interface
// that re-declares every PostingsWriterBase method (Go has no virtual call
// through embedded interfaces) plus the push-API methods. Concrete codec
// writers implement this interface directly. A PushPostingsWriterBase
// implementation also satisfies PostingsWriterBase; the WriteTerm helper
// further consumes a TermsEnum and drives the push API automatically.
//
// The push API is used by block-tree-style term dictionaries: the dictionary
// pulls each (doc, freq, position, payload) tuple from a TermsEnum and pushes
// it into the writer, which buffers, compresses, and flushes blocks of
// postings to its own files (.doc/.pos/.pay in the default codec).
type PushPostingsWriterBase interface {
	PostingsWriterBase

	// StartDoc begins a new document in the current term's posting list.
	// freq is the in-document term frequency.
	StartDoc(docID, freq int) error

	// AddPosition records a position occurrence for the current document.
	// startOffset/endOffset are character offsets (use -1 when offsets are
	// not stored); payload may be nil.
	AddPosition(position int, payload []byte, startOffset, endOffset int) error

	// FinishDoc finalizes the current document. Called after every
	// AddPosition call for the doc has been made.
	FinishDoc() error
}

// WriteTerm pulls every (doc, freq, position, offset, payload) tuple from
// postingsEnum and pushes it into writer following the Lucene 10.4.0
// PushPostingsWriterBase.writeTerm protocol:
//
//   - The caller is responsible for invoking writer.StartTerm before WriteTerm
//     and writer.FinishTerm after WriteTerm.
//   - WriteTerm iterates postingsEnum's documents, calling StartDoc/FinishDoc
//     for each one. When indexHasPositions is true it also iterates positions
//     and forwards each one through AddPosition, including offsets and
//     payload data when present.
//   - Returns the number of documents pushed and the total term frequency
//     accumulated across all docs. totalTermFreq is -1 when indexHasFreqs is
//     false (DOCS-only field), matching Java's writeTerm return contract.
//     The caller must set state.TotalTermFreq = totalTermFreq and
//     state.DocFreq = docCount before calling FinishTerm.
//
// indexHasFreqs/indexHasPositions/indexHasOffsets/indexHasPayloads must agree
// with the field's IndexOptions; the helper does not consult fieldInfo to
// avoid a coupling that breaks in shadow tests.
func WriteTerm(
	writer PushPostingsWriterBase,
	postingsEnum index.PostingsEnum,
	indexHasFreqs, indexHasPositions, indexHasOffsets, indexHasPayloads bool,
) (docCount int, totalTermFreq int64, err error) {
	if postingsEnum == nil {
		return 0, 0, fmt.Errorf("WriteTerm: nil postingsEnum")
	}

	var ttf int64
	for {
		docID, nerr := postingsEnum.NextDoc()
		if nerr != nil {
			return docCount, 0, fmt.Errorf("WriteTerm: NextDoc: %w", nerr)
		}
		if docID == index.NO_MORE_DOCS {
			break
		}

		var freq int
		if indexHasFreqs {
			freq, nerr = postingsEnum.Freq()
			if nerr != nil {
				return docCount, 0, fmt.Errorf("WriteTerm: Freq(doc=%d): %w", docID, nerr)
			}
			ttf += int64(freq)
		} else {
			freq = -1
		}

		if nerr = writer.StartDoc(docID, freq); nerr != nil {
			return docCount, 0, fmt.Errorf("WriteTerm: StartDoc(doc=%d, freq=%d): %w", docID, freq, nerr)
		}

		if indexHasPositions {
			for i := 0; i < freq; i++ {
				pos, perr := postingsEnum.NextPosition()
				if perr != nil {
					return docCount, 0, fmt.Errorf("WriteTerm: NextPosition(doc=%d): %w", docID, perr)
				}
				// NO_MORE_POSITIONS is a defensive break: well-formed
				// posting lists yield exactly freq positions per doc.
				if pos == index.NO_MORE_POSITIONS {
					break
				}

				var startOffset, endOffset int = -1, -1
				if indexHasOffsets {
					startOffset, perr = postingsEnum.StartOffset()
					if perr != nil {
						return docCount, 0, fmt.Errorf("WriteTerm: StartOffset(doc=%d, pos=%d): %w", docID, pos, perr)
					}
					endOffset, perr = postingsEnum.EndOffset()
					if perr != nil {
						return docCount, 0, fmt.Errorf("WriteTerm: EndOffset(doc=%d, pos=%d): %w", docID, pos, perr)
					}
				}

				var payload []byte
				if indexHasPayloads {
					payload, perr = postingsEnum.GetPayload()
					if perr != nil {
						return docCount, 0, fmt.Errorf("WriteTerm: GetPayload(doc=%d, pos=%d): %w", docID, pos, perr)
					}
				}

				if perr = writer.AddPosition(pos, payload, startOffset, endOffset); perr != nil {
					return docCount, 0, fmt.Errorf("WriteTerm: AddPosition(doc=%d, pos=%d): %w", docID, pos, perr)
				}
			}
		}

		if nerr = writer.FinishDoc(); nerr != nil {
			return docCount, 0, fmt.Errorf("WriteTerm: FinishDoc(doc=%d): %w", docID, nerr)
		}
		docCount++
	}

	if indexHasFreqs {
		return docCount, ttf, nil
	}
	return docCount, -1, nil
}

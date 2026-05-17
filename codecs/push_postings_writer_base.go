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
//   - Returns the number of documents pushed (matches Lucene's int return).
//
// indexHasPositions/indexHasOffsets/indexHasPayloads must agree with the
// field's IndexOptions; the helper does not consult fieldInfo to avoid a
// coupling that breaks in shadow tests.
func WriteTerm(
	writer PushPostingsWriterBase,
	postingsEnum index.PostingsEnum,
	indexHasPositions, indexHasOffsets, indexHasPayloads bool,
) (int, error) {
	if postingsEnum == nil {
		return 0, fmt.Errorf("WriteTerm: nil postingsEnum")
	}

	docCount := 0
	for {
		docID, err := postingsEnum.NextDoc()
		if err != nil {
			return docCount, fmt.Errorf("WriteTerm: NextDoc: %w", err)
		}
		if docID == index.NO_MORE_DOCS {
			break
		}

		freq, err := postingsEnum.Freq()
		if err != nil {
			return docCount, fmt.Errorf("WriteTerm: Freq(doc=%d): %w", docID, err)
		}

		if err := writer.StartDoc(docID, freq); err != nil {
			return docCount, fmt.Errorf("WriteTerm: StartDoc(doc=%d, freq=%d): %w", docID, freq, err)
		}

		if indexHasPositions {
			for i := 0; i < freq; i++ {
				pos, err := postingsEnum.NextPosition()
				if err != nil {
					return docCount, fmt.Errorf("WriteTerm: NextPosition(doc=%d): %w", docID, err)
				}
				// NO_MORE_POSITIONS is a defensive break: well-formed
				// posting lists yield exactly freq positions per doc.
				if pos == index.NO_MORE_POSITIONS {
					break
				}

				var startOffset, endOffset int = -1, -1
				if indexHasOffsets {
					startOffset, err = postingsEnum.StartOffset()
					if err != nil {
						return docCount, fmt.Errorf("WriteTerm: StartOffset(doc=%d, pos=%d): %w", docID, pos, err)
					}
					endOffset, err = postingsEnum.EndOffset()
					if err != nil {
						return docCount, fmt.Errorf("WriteTerm: EndOffset(doc=%d, pos=%d): %w", docID, pos, err)
					}
				}

				var payload []byte
				if indexHasPayloads {
					payload, err = postingsEnum.GetPayload()
					if err != nil {
						return docCount, fmt.Errorf("WriteTerm: GetPayload(doc=%d, pos=%d): %w", docID, pos, err)
					}
				}

				if err := writer.AddPosition(pos, payload, startOffset, endOffset); err != nil {
					return docCount, fmt.Errorf("WriteTerm: AddPosition(doc=%d, pos=%d): %w", docID, pos, err)
				}
			}
		}

		if err := writer.FinishDoc(); err != nil {
			return docCount, fmt.Errorf("WriteTerm: FinishDoc(doc=%d): %w", docID, err)
		}
		docCount++
	}
	return docCount, nil
}

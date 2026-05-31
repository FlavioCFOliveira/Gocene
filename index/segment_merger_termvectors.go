// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "fmt"

// mergeTermVectors merges the per-document term vectors of every source segment
// into the new segment. It walks the live documents in merged-docID order and
// replays each document's term vectors (field -> term -> positions/offsets/
// payloads) through the codec's TermVectorsWriter — the net effect of Lucene's
// TermVectorsWriter.merge(MergeState) (rmp #14/#114).
//
// Returns the number of documents written (every live doc, including those with
// no term vectors, gets a StartDocument/FinishDocument pair).
func (sm *SegmentMerger) mergeTermVectors() (int, error) {
	if sm.codec == nil || sm.codec.TermVectorsFormat() == nil {
		return sm.MergeState.SegmentInfo.DocCount(), nil
	}
	if sm.MergeState.DocMaps == nil {
		sm.buildDocMaps()
	}

	state := &SegmentWriteState{
		Directory:     sm.directory,
		SegmentInfo:   sm.MergeState.SegmentInfo,
		FieldInfos:    sm.MergeState.MergeFieldInfos,
		SegmentSuffix: "",
	}
	writer, err := sm.codec.TermVectorsFormat().VectorsWriter(state)
	if err != nil {
		return 0, fmt.Errorf("index: merge term vectors: open writer: %w", err)
	}
	defer writer.Close()

	total := 0
	for i, reader := range sm.MergeState.Readers {
		if reader == nil {
			continue
		}
		maxDoc := sm.MergeState.MaxDocs[i]
		live := sm.MergeState.LiveDocs[i]
		for docID := 0; docID < maxDoc; docID++ {
			if live != nil && !live.Get(docID) {
				continue
			}
			fields, err := reader.GetTermVectors(docID)
			if err != nil {
				return 0, fmt.Errorf("index: merge term vectors: read doc %d of reader %d: %w", docID, i, err)
			}
			if err := sm.writeDocTermVectors(writer, fields); err != nil {
				return 0, err
			}
			total++
		}
	}
	return total, nil
}

// tvOcc is one occurrence of a term within a document's term vector.
type tvOcc struct {
	pos     int
	startO  int
	endO    int
	payload []byte
}

// tvTerm is one term's collected term-vector data for a single document.
type tvTerm struct {
	bytes []byte
	occs  []tvOcc
}

// writeDocTermVectors replays one document's term vectors through writer. It
// collects each field's terms into memory first so the exact term count is
// known for StartField, then drives the StartTerm/AddPosition protocol.
func (sm *SegmentMerger) writeDocTermVectors(writer TermVectorsWriter, fields Fields) error {
	// Enumerate the document's term-vector fields.
	var fieldNames []string
	if fields != nil {
		it, err := fields.Iterator()
		if err != nil {
			return fmt.Errorf("index: merge term vectors: field iterator: %w", err)
		}
		if it != nil {
			for {
				name, err := it.Next()
				if err != nil {
					return err
				}
				if name == "" {
					break
				}
				fieldNames = append(fieldNames, name)
			}
		}
	}

	if err := writer.StartDocument(len(fieldNames)); err != nil {
		return fmt.Errorf("index: merge term vectors: start document: %w", err)
	}

	for _, name := range fieldNames {
		terms, err := fields.Terms(name)
		if err != nil {
			return err
		}
		if terms == nil {
			// Field listed but no terms: emit an empty field so the
			// StartDocument count stays consistent.
			if err := writer.StartField(sm.MergeState.MergeFieldInfos.GetByName(name), 0, false, false, false); err != nil {
				return err
			}
			if err := writer.FinishField(); err != nil {
				return err
			}
			continue
		}
		hasPos := terms.HasPositions()
		hasOff := terms.HasOffsets()
		hasPay := terms.HasPayloads()

		// Collect the field's terms (so StartField gets the exact count). The
		// per-occurrence positions/offsets are only available when the term
		// vectors TermsEnum exposes a Postings enum; until that read gap is
		// closed (rmp #121) collectTVTerms falls back to the term frequency
		// (TotalTermFreq) with no positions, so the merged vectors preserve
		// terms and freqs but drop positions/offsets.
		collected, gotPositions, gotOffsets, err := collectTVTerms(terms, hasPos, hasOff, hasPay)
		if err != nil {
			return err
		}
		hasPos = hasPos && gotPositions
		hasOff = hasOff && gotOffsets

		fieldInfo := sm.MergeState.MergeFieldInfos.GetByName(name)
		if err := writer.StartField(fieldInfo, len(collected), hasPos, hasOff, hasPay); err != nil {
			return fmt.Errorf("index: merge term vectors: start field %q: %w", name, err)
		}
		for _, term := range collected {
			if err := writer.StartTerm(term.bytes); err != nil {
				return err
			}
			for _, occ := range term.occs {
				if err := writer.AddPosition(occ.pos, occ.startO, occ.endO, occ.payload); err != nil {
					return err
				}
			}
			if err := writer.FinishTerm(); err != nil {
				return err
			}
		}
		if err := writer.FinishField(); err != nil {
			return err
		}
	}

	return writer.FinishDocument()
}

// collectTVTerms reads one term-vector field's terms and per-term occurrences
// into memory. It returns the collected terms plus whether positions/offsets
// were actually available from the source (false when the term vectors
// TermsEnum exposes no Postings enum — the rmp #121 read gap — in which case
// the term frequency is taken from TotalTermFreq and no positions are emitted).
func collectTVTerms(terms Terms, hasPos, hasOff, hasPay bool) (out []tvTerm, gotPositions, gotOffsets bool, err error) {
	te, err := terms.GetIterator()
	if err != nil {
		return nil, false, false, err
	}
	if te == nil {
		return nil, false, false, nil
	}
	for {
		term, err := te.Next()
		if err != nil {
			return nil, false, false, err
		}
		if term == nil {
			break
		}
		pe, err := te.Postings(PostingsFlagAll)
		if err != nil {
			return nil, false, false, err
		}
		t := tvTerm{bytes: append([]byte(nil), termBytesOf(term)...)}

		if pe != nil {
			// Positions/offsets are readable for this term vector.
			d, err := pe.NextDoc()
			if err != nil {
				return nil, false, false, err
			}
			if d >= 0 {
				freq, err := pe.Freq()
				if err != nil {
					return nil, false, false, err
				}
				if freq < 1 {
					freq = 1
				}
				for k := 0; k < freq; k++ {
					occ := tvOcc{pos: -1, startO: -1, endO: -1}
					if hasPos {
						gotPositions = true
						if occ.pos, err = pe.NextPosition(); err != nil {
							return nil, false, false, err
						}
					}
					if hasOff {
						gotOffsets = true
						if occ.startO, err = pe.StartOffset(); err != nil {
							return nil, false, false, err
						}
						if occ.endO, err = pe.EndOffset(); err != nil {
							return nil, false, false, err
						}
					}
					if hasPay {
						p, err := pe.GetPayload()
						if err != nil {
							return nil, false, false, err
						}
						if len(p) > 0 {
							occ.payload = append([]byte(nil), p...)
						}
					}
					t.occs = append(t.occs, occ)
				}
			}
		} else {
			// Read gap (rmp #121): no Postings enum for this term vector.
			// Preserve the term and its frequency via TotalTermFreq; emit
			// position-less occurrences (the StartField flags are downgraded
			// to no-positions/offsets accordingly by the caller).
			freq, err := te.TotalTermFreq()
			if err != nil {
				return nil, false, false, err
			}
			if freq < 1 {
				freq = 1
			}
			for k := int64(0); k < freq; k++ {
				t.occs = append(t.occs, tvOcc{pos: -1, startO: -1, endO: -1})
			}
		}
		out = append(out, t)
	}
	return out, gotPositions, gotOffsets, nil
}

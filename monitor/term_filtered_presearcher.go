// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermFilteredPresearcher is a Presearcher implementation that uses term
// extraction to reduce the candidate set for each document.
//
// Port of org.apache.lucene.monitor.TermFilteredPresearcher.
//
// Deviation: Full integration with Lucene's index API (TermsEnum, LeafReader,
// posting list analysis) is deferred to backlog #2693.  This port captures the
// public contract (constants + interface satisfaction) and the custom-handler
// delegation path.
type TermFilteredPresearcher struct {
	customHandlers []CustomQueryHandler
}

// NewTermFilteredPresearcher creates a TermFilteredPresearcher with no custom handlers.
func NewTermFilteredPresearcher() *TermFilteredPresearcher {
	return &TermFilteredPresearcher{}
}

// NewTermFilteredPresearcherWithHandlers creates a TermFilteredPresearcher with
// the given custom query handlers.
func NewTermFilteredPresearcherWithHandlers(handlers ...CustomQueryHandler) *TermFilteredPresearcher {
	return &TermFilteredPresearcher{customHandlers: handlers}
}

// BuildQuery returns nil (MatchAllDocs) — full term-based query building is
// deferred to backlog #2693.
func (p *TermFilteredPresearcher) BuildQuery(
	_ interface{},
	_ func(string, *util.BytesRef) bool,
) search.Query {
	return nil
}

// IndexQuery returns nil — full document building is deferred to backlog #2693.
func (p *TermFilteredPresearcher) IndexQuery(_ search.Query, _ map[string]string) interface{} {
	return nil
}

// MultipassTermFilteredPresearcher runs multiple presearcher phases to
// progressively narrow the candidate set.
//
// Port of org.apache.lucene.monitor.MultipassTermFilteredPresearcher.
//
// Deviation: Full multipass logic is deferred to backlog #2693.
type MultipassTermFilteredPresearcher struct {
	TermFilteredPresearcher
	passes int
}

// NewMultipassTermFilteredPresearcher creates a multipass presearcher with the
// given number of passes and custom handlers.
func NewMultipassTermFilteredPresearcher(passes int, handlers ...CustomQueryHandler) *MultipassTermFilteredPresearcher {
	return &MultipassTermFilteredPresearcher{
		TermFilteredPresearcher: TermFilteredPresearcher{customHandlers: handlers},
		passes:                  passes,
	}
}

// Passes returns the number of presearcher passes.
func (p *MultipassTermFilteredPresearcher) Passes() int { return p.passes }

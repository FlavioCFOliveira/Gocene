// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// IndexDictionary mirrors org.apache.lucene.codecs.uniformsplit.IndexDictionary.
type IndexDictionary interface {
	// Get returns the BlockHeader for the given term.
	Get(term *util.BytesRef) (*BlockHeader, error)
}

// Browser is a simple dictionary browser.
type Browser struct {
	// implementation details...
}

// BrowserSupplier provides Browser instances.
type BrowserSupplier struct {
	// implementation details...
}

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uhighlight

// PassageFormatter renders a set of top passages into a human-readable
// snippet string. Mirrors org.apache.lucene.search.uhighlight.PassageFormatter.
type PassageFormatter interface {
	// Format renders passages (sorted in document order) against the
	// original field content. Returns the rendered snippet string.
	Format(passages []*Passage, content string) string
}

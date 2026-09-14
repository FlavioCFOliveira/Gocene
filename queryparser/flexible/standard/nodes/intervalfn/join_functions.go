// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervalfn

import "strings"

// joinFunctions joins the String() values of sources with a single space.
//
// Mirrors the
// sources.stream().map(IntervalFunction::toString).collect(Collectors.joining(" "))
// idiom that Ordered, Unordered, Phrase, Or and AtLeast all use in their
// toString().
func joinFunctions(sources []IntervalFunction) string {
	parts := make([]string, len(sources))
	for i, src := range sources {
		parts[i] = src.String()
	}
	return strings.Join(parts, " ")
}

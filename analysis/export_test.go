// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// CachedTokenCount exposes the package-private
// GraphTokenFilter.cachedTokenCount() to the external test package
// analysis_test, whose tests live in package org.apache.lucene.analysis in
// Java and call it directly.
func (f *GraphTokenFilter) CachedTokenCount() int {
	return f.cachedTokenCount()
}

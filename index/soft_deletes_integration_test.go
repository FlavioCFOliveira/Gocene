// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

// Soft deletes integration tests are skipped because:
//   - IndexWriterConfig.SetSoftDeletesEnabled is not yet implemented.
//   - IndexWriter.SoftUpdateDocument is not yet implemented.

import "testing"

func TestSoftDeletesIntegration_BasicSoftDelete(t *testing.T) {
	t.Skip("Skipping: SoftUpdateDocument not yet implemented")
}

func TestSoftDeletesIntegration_Purging(t *testing.T) {
	t.Skip("Skipping: SoftUpdateDocument not yet implemented")
}

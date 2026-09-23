// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build !gocene_monsters

package index_test

// testNightly renders LuceneTestCase.TEST_NIGHTLY. Nightly runs are selected
// by the repository's gocene_monsters build tag.
const testNightly = false

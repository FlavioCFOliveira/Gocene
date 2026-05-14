// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// -----------------------------------------------------------------------------
// PORT NOTE (intentional divergence from Java):
//
// Java's @SuppressForbidden annotation feeds Gradle's forbiddenapis
// plugin: the build inspects class files and skips reporting on
// elements marked with this annotation. Go has no equivalent
// annotation machinery and no forbidden-API gating in the Gocene
// build, so a faithful port is impossible — and unnecessary.
//
// The Go translation is purely conventional:
//
//   - Use the //gocene:suppress-forbidden=<reason> comment directive
//     immediately above the symbol you would have annotated.
//   - When reviewing code, treat the directive as documentation: it
//     must record a reason, exactly as the Java annotation required.
//
// No build-time enforcement is wired here. The constant
// SuppressForbiddenDirective is exposed so tooling that wants to grep
// for the marker has a stable string to look for.
// -----------------------------------------------------------------------------

package util

// SuppressForbiddenDirective is the canonical comment prefix used to
// mark a Go symbol as "would have carried @SuppressForbidden in
// Java". The expected layout is one line directly above the symbol:
//
//	//gocene:suppress-forbidden=reason goes here
//	func legacyApiCall() { ... }
//
// Tooling that wants to extract these markers can match the prefix
// against this constant.
const SuppressForbiddenDirective = "//gocene:suppress-forbidden="

// SuppressForbidden documents that a Go function/type/variable would
// have been annotated with the Java @SuppressForbidden annotation.
//
// It is a no-op at runtime; its only purpose is to make the
// "intentional Lucene divergence" explicit when grepping for forbidden
// API conventions inside Gocene. Callers may invoke it with the same
// reason string they would have passed to the Java annotation.
//
// Example:
//
//	//gocene:suppress-forbidden=System.out is forbidden, but log.go is the boot path
//	func bootstrapLog() { ... }
//	var _ = util.SuppressForbidden("System.out: bootstrap path before logger is wired")
func SuppressForbidden(reason string) string { return reason }

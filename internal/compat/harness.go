// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package compat exposes a thin Go wrapper around the Java/Lucene fixture
// harness shipped in tools/lucene-fixtures/. It is the only sanctioned way
// for Gocene's compatibility tests to ask Lucene 10.4.0 to produce or
// verify a binary artefact.
//
// The harness is located at runtime through the LUCENE_FIXTURES_JAR
// environment variable; if unset, the helper looks for the conventional
// build output under <repo-root>/tools/lucene-fixtures/target/. When the
// jar cannot be found the helper returns ErrHarnessMissing so callers can
// skip gracefully rather than fail CI before the harness is built.
package compat

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrHarnessMissing is returned by Locate and the public helpers when the
// Java harness jar is not reachable. Tests that exercise round-trip
// compatibility should treat this as a "skip" signal rather than a failure.
var ErrHarnessMissing = errors.New("lucene-fixtures harness jar not found (set LUCENE_FIXTURES_JAR or run 'make -f tools/lucene-fixtures/Makefile harness-build')")

// Locate returns the absolute path of the harness jar.
//
// Resolution order:
//  1. LUCENE_FIXTURES_JAR (if it points to an existing file).
//  2. <git repo root>/tools/lucene-fixtures/target/lucene-fixtures.jar.
//
// Locate never invokes the JVM; it only inspects the filesystem.
func Locate() (string, error) {
	if v := os.Getenv("LUCENE_FIXTURES_JAR"); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v, nil
		}
	}
	repo, err := repoRoot()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrHarnessMissing, err)
	}
	jar := filepath.Join(repo, "tools", "lucene-fixtures", "target", "lucene-fixtures.jar")
	if _, err := os.Stat(jar); err == nil {
		return jar, nil
	}
	return "", ErrHarnessMissing
}

// List returns the names of every scenario registered in the harness, in
// registration order.
func List() ([]string, error) {
	jar, err := Locate()
	if err != nil {
		return nil, err
	}
	out, err := runJar(jar, "list")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// The CLI prints one name per line. Defensive: take the first
		// tab-separated field in case future versions append metadata.
		name := strings.SplitN(line, "\t", 2)[0]
		names = append(names, name)
	}
	return names, nil
}

// Generate asks the harness to produce the given scenario with the given seed
// into a fresh temporary directory and returns the directory path. The
// caller is responsible for removing it (or relying on t.TempDir for tests).
func Generate(scenario string, seed int64) (string, error) {
	dir, err := os.MkdirTemp("", "gocene-fixture-"+scenario+"-")
	if err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}
	if err := GenerateInto(scenario, seed, dir); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}

// GenerateInto is the explicit-target variant of Generate.
func GenerateInto(scenario string, seed int64, target string) error {
	jar, err := Locate()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("mkdir target: %w", err)
	}
	_, err = runJar(jar, "gen", scenario, strconv.FormatInt(seed, 10), target)
	return err
}

// Verify invokes the harness verifier on a path produced by Gocene.
func Verify(scenario string, seed int64, source string) error {
	jar, err := Locate()
	if err != nil {
		return err
	}
	_, err = runJar(jar, "verify", scenario, strconv.FormatInt(seed, 10), source)
	return err
}

// runJar executes "java -jar <jar> <args...>" and returns its combined output.
// Non-zero exit codes are surfaced as Go errors with the full output attached.
func runJar(jar string, args ...string) (string, error) {
	cmd := exec.Command("java", append([]string{"-jar", jar}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("java -jar %s %s failed: %w (stderr: %s)",
			filepath.Base(jar), strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// repoRoot returns the working tree root via `git rev-parse --show-toplevel`.
func repoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

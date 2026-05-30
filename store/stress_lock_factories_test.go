// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

// TestStressLockFactories is the Go port of
// org.apache.lucene.store.TestStressLockFactories.
//
// The Java original spawns a fresh JVM per client and lets the in-process
// LockVerifyServer arbitrate between them, exercising the cross-process
// guarantees of NativeFSLockFactory and SimpleFSLockFactory. Doing the same
// thing in Go requires re-exec'ing the test binary: TestMain intercepts the
// "GOCENE_STRESS_LOCK_FACTORIES_CHILD" environment variable, dispatches into
// [LockStressMain], and exits before the testing framework ever runs. The
// parent test then drives [LockVerifyServerRun] in-process and spawns N child
// processes through os.Args[0].
//
// See [LockStressMain] / [LockVerifyServerRun] for the wire-protocol details,
// which are byte-for-byte compatible with Lucene's LockStressTest /
// LockVerifyServer pair.

// stressChildEnvVar is consumed by [TestMain] to decide whether the current
// invocation is the parent test process or a re-exec'd stress client. The
// value is the verbatim argv that should be handed to [LockStressMain], one
// argument per line (newline-separated, no trailing newline). Using a single
// env var avoids polluting Args with non-test flags that would confuse the
// testing framework if dispatch ever missed.
const stressChildEnvVar = "GOCENE_STRESS_LOCK_FACTORIES_CHILD"

// TestMain dispatches re-exec'd child invocations into [LockStressMain] before
// any test runs. In the parent process the env var is unset, so we fall
// through to the standard m.Run() and exit with its status. Keeping this here
// (rather than in a dedicated file) makes the dependency obvious to anyone
// editing TestStressLockFactories.
func TestMain(m *testing.M) {
	if payload, ok := os.LookupEnv(stressChildEnvVar); ok {
		os.Exit(LockStressMain(decodeChildArgs(payload)))
	}
	os.Exit(m.Run())
}

// decodeChildArgs reverses [encodeChildArgs]. Empty input yields a nil slice,
// matching what [LockStressMain] expects for a usage error.
func decodeChildArgs(payload string) []string {
	if payload == "" {
		return nil
	}
	// Hand-rolled split to avoid pulling in the strings package for one call.
	out := make([]string, 0, 7)
	start := 0
	for i := 0; i < len(payload); i++ {
		if payload[i] == '\n' {
			out = append(out, payload[start:i])
			start = i + 1
		}
	}
	out = append(out, payload[start:])
	return out
}

// encodeChildArgs serialises a stress-test argv into the single
// newline-delimited string consumed by [decodeChildArgs]. None of the
// arguments produced by the parent test ever contain a newline, so this
// simple framing is safe.
func encodeChildArgs(args []string) string {
	total := 0
	for i, a := range args {
		total += len(a)
		if i > 0 {
			total++
		}
	}
	buf := make([]byte, 0, total)
	for i, a := range args {
		if i > 0 {
			buf = append(buf, '\n')
		}
		buf = append(buf, a...)
	}
	return string(buf)
}

// stressClientCount mirrors Lucene's `TEST_NIGHTLY ? 5 : 2`; we never run
// nightly here, so the smaller cohort applies.
const stressClientCount = 2

// stressRounds mirrors Lucene's `(TEST_NIGHTLY ? 30000 : 500) * RANDOM_MULTIPLIER`
// with RANDOM_MULTIPLIER fixed at 1, matching the default Gocene test
// configuration.
const stressRounds = 500

// stressDelayMS matches Lucene's `final int delay = 1;`.
const stressDelayMS = 1

// stressClientTimeout caps how long the parent waits for any one child to
// exit cleanly. Lucene uses 15 seconds; we keep the same budget. Note that
// 500 rounds * 2 sleeps * 1ms = ~1s of pure sleep per client, leaving
// generous headroom for filesystem and TCP jitter.
const stressClientTimeout = 15 * time.Second

// stressChildRecord pairs a spawned child process with the paths of its
// captured stdout / stderr, so the parent can correlate exit-code failures
// with what the child actually printed before dying.
type stressChildRecord struct {
	idx     int
	cmd     *exec.Cmd
	outPath string
	errPath string
}

// runStressImpl drives one (server, N clients) cycle for the given factory
// symbolic name (the form understood by [DefaultLockStressFactoryResolver]).
// It is the Go analogue of Lucene's `runImpl(Class<? extends LockFactory>)`.
func runStressImpl(t *testing.T, factoryName string) {
	t.Helper()

	if _, ok := os.LookupEnv(stressChildEnvVar); ok {
		// Defensive: a re-exec'd child should have been intercepted in
		// TestMain. Reaching this path means the dispatch was missed.
		t.Fatalf("re-exec child reached test body; TestMain dispatch is broken")
	}

	const host = "127.0.0.1"

	binary, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test binary: %v", err)
	}

	lockDir := t.TempDir()
	logDir := t.TempDir()

	children := make([]*stressChildRecord, 0, stressClientCount)
	var spawnErr error

	err = LockVerifyServerRun(host, stressClientCount, func(addr net.Addr) {
		tcpAddr, ok := addr.(*net.TCPAddr)
		if !ok {
			spawnErr = fmt.Errorf("expected *net.TCPAddr, got %T", addr)
			return
		}
		for i := 0; i < stressClientCount; i++ {
			rec, err := spawnStressClient(binary, lockDir, logDir, host, tcpAddr.Port, factoryName, i)
			if err != nil {
				spawnErr = fmt.Errorf("spawn client %d: %w", i, err)
				return
			}
			children = append(children, rec)
		}
	})

	if spawnErr != nil {
		dumpChildLogs(t, children)
		t.Fatalf("server failed during spawn: %v", spawnErr)
	}
	if err != nil {
		dumpChildLogs(t, children)
		t.Fatalf("server failed: %v", err)
	}

	// Reap every child. We always invoke Wait on all of them so the OS
	// release-resources path runs even when some children timed out.
	var (
		waitErrs []error
		killWG   sync.WaitGroup
	)

	for _, c := range children {
		c := c
		done := make(chan error, 1)
		go func() { done <- c.cmd.Wait() }()

		select {
		case waitErr := <-done:
			if waitErr != nil {
				waitErrs = append(waitErrs, fmt.Errorf("client %d: %w", c.idx, waitErr))
				dumpSingleChild(t, c)
			}
		case <-time.After(stressClientTimeout):
			waitErrs = append(waitErrs, fmt.Errorf("client %d (pid %d): did not finish within %s", c.idx, c.cmd.Process.Pid, stressClientTimeout))
			dumpSingleChild(t, c)
			killWG.Add(1)
			go func() {
				defer killWG.Done()
				_ = c.cmd.Process.Kill()
				<-done
			}()
		}
	}

	killWG.Wait()

	if len(waitErrs) > 0 {
		t.Fatalf("stress run failed: %v", errors.Join(waitErrs...))
	}
}

// spawnStressClient re-execs the test binary in "child" mode and wires its
// stdout / stderr to per-client log files under logDir, mirroring Lucene's
// `out-<i>.txt` / `err-<i>.txt` redirection.
func spawnStressClient(binary, lockDir, logDir, host string, port int, factoryName string, idx int) (*stressChildRecord, error) {
	args := []string{
		strconv.Itoa(idx),
		host,
		strconv.Itoa(port),
		factoryName,
		lockDir,
		strconv.Itoa(stressDelayMS),
		strconv.Itoa(stressRounds),
	}

	outPath := filepath.Join(logDir, fmt.Sprintf("out-%d.txt", idx))
	errPath := filepath.Join(logDir, fmt.Sprintf("err-%d.txt", idx))

	outFile, err := os.Create(outPath)
	if err != nil {
		return nil, fmt.Errorf("create stdout log: %w", err)
	}
	errFile, err := os.Create(errPath)
	if err != nil {
		_ = outFile.Close()
		return nil, fmt.Errorf("create stderr log: %w", err)
	}

	// Hand the binary a no-op test selector. We must run *something* under
	// the testing framework's matcher to avoid `-run` selecting the
	// well-known top-level tests on the child; TestMain intercepts before
	// any test runs anyway, but the explicit selector keeps the contract
	// obvious and prevents stray output if dispatch ever regresses.
	cmd := exec.Command(binary, "-test.run=^$")
	cmd.Env = append(os.Environ(), stressChildEnvVar+"="+encodeChildArgs(args))
	cmd.Stdout = outFile
	cmd.Stderr = errFile
	// Inherit stdin like Lucene's Redirect.INHERIT.
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		_ = outFile.Close()
		_ = errFile.Close()
		return nil, fmt.Errorf("start: %w", err)
	}

	// We can close the file handles in the parent now; the OS keeps them
	// open in the child for the lifetime of the process.
	_ = outFile.Close()
	_ = errFile.Close()

	return &stressChildRecord{idx: idx, cmd: cmd, outPath: outPath, errPath: errPath}, nil
}

// dumpChildLogs writes every child's captured stdout/stderr into the test
// log, matching what Lucene's runImpl prints on failure.
func dumpChildLogs(t *testing.T, children []*stressChildRecord) {
	t.Helper()
	for _, c := range children {
		dumpSingleChild(t, c)
	}
}

func dumpSingleChild(t *testing.T, c *stressChildRecord) {
	t.Helper()
	if c == nil {
		return
	}
	pid := -1
	if c.cmd != nil && c.cmd.Process != nil {
		pid = c.cmd.Process.Pid
	}
	if data, err := os.ReadFile(c.errPath); err == nil && len(data) > 0 {
		t.Logf("stderr for pid %d (client %d):\n%s", pid, c.idx, data)
	}
	if data, err := os.ReadFile(c.outPath); err == nil && len(data) > 0 {
		t.Logf("stdout for pid %d (client %d):\n%s", pid, c.idx, data)
	}
}

// TestStressLockFactoriesNativeFS is the Go port of
// testNativeFSLockFactory().
func TestStressLockFactoriesNativeFS(t *testing.T) {
	if testing.Short() {
		t.Fatal("skipping multi-process lock stress test in -short mode")
	}
	runStressImpl(t, "native")
}

// TestStressLockFactoriesSimpleFS is the Go port of
// testSimpleFSLockFactory(). On Windows, advisory file locks behave
// differently from POSIX; the upstream test suite has historically
// suppressed the SimpleFS variant in similar scenarios. We keep the test
// enabled on POSIX hosts and skip on Windows to avoid spurious failures.
func TestStressLockFactoriesSimpleFS(t *testing.T) {
	if testing.Short() {
		t.Fatal("skipping multi-process lock stress test in -short mode")
	}
	if runtime.GOOS == "windows" {
		t.Fatal("SimpleFSLockFactory stress test is not exercised on windows")
	}
	runStressImpl(t, "simple")
}

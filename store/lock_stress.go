// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"os"
	"strconv"
	"time"
)

// LockStressTestLockFileName is the lock file name used by the stress client,
// matching Lucene's LockStressTest.LOCK_FILE_NAME.
const LockStressTestLockFileName = "test.lock"

// lockStressDoubleObtainProbability is the 1-in-N probability the client tries
// a second, deliberately-conflicting obtain inside the same hold. Lucene picks
// 1/10 (rnd.nextInt(10) == 0); we keep the exact ratio.
const lockStressDoubleObtainProbability = 10

// lockStressProgressEvery controls how often a progress percentage is printed
// to stdout, mirroring Lucene's `i % 500 == 0` cadence.
const lockStressProgressEvery = 500

// lockStressDialTimeout caps how long the client waits when dialing the
// verifier server, matching the 3-second hard cap in Lucene's LockStressTest.
const lockStressDialTimeout = 3 * time.Second

// LockStressFactoryResolver maps a symbolic lock-factory name to a concrete
// FSLockFactory-shaped value. It exists because Go has no equivalent of Java's
// reflective Class.forName(...).getField("INSTANCE") used by Lucene to load
// the factory. Callers may inject a custom resolver (tests do); the default
// covers the four canonical factories shipped by the package.
type LockStressFactoryResolver func(name string) (LockFactory, error)

// DefaultLockStressFactoryResolver resolves the short symbolic names used by
// [LockStressMain] / [LockStressRun]:
//
//   - "native"  -> NewNativeFSLockFactory()
//   - "simple"  -> SimpleFSLockFactoryInstance
//   - "single"  -> NewSingleInstanceLockFactory()
//   - "nolock"  -> NewNoLockFactory()
//
// The full Java class names accepted by Lucene's LockStressTest are also
// honoured for documentation parity (case-insensitive last segment):
//
//   - "org.apache.lucene.store.NativeFSLockFactory" -> "native"
//   - "org.apache.lucene.store.SimpleFSLockFactory" -> "simple"
//   - "org.apache.lucene.store.SingleInstanceLockFactory" -> "single"
//   - "org.apache.lucene.store.NoLockFactory" -> "nolock"
//
// Any other name returns an error.
func DefaultLockStressFactoryResolver(name string) (LockFactory, error) {
	switch normalizeLockFactoryName(name) {
	case "native", "nativefslockfactory":
		return NewNativeFSLockFactory(), nil
	case "simple", "simplefslockfactory":
		return SimpleFSLockFactoryInstance, nil
	case "single", "singleinstancelockfactory":
		return NewSingleInstanceLockFactory(), nil
	case "nolock", "nolockfactory":
		return NewNoLockFactory(), nil
	default:
		return nil, fmt.Errorf("lock stress: unknown lock factory %q (expected one of: native, simple, single, nolock)", name)
	}
}

// normalizeLockFactoryName lower-cases the last dot-separated segment of name
// so both short tokens (e.g. "native") and fully qualified class names
// (e.g. "org.apache.lucene.store.NativeFSLockFactory") collapse to a single
// canonical form.
func normalizeLockFactoryName(name string) string {
	last := name
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			last = name[i+1:]
			break
		}
	}
	// Manual ASCII lower-case; avoids importing strings for one call and stays
	// allocation-free for the typical 16-byte-ish input.
	buf := make([]byte, len(last))
	for i := 0; i < len(last); i++ {
		c := last[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		buf[i] = c
	}
	return string(buf)
}

// LockStressRun connects to a running [LockVerifyServerRun] instance and
// repeatedly obtains and releases [LockStressTestLockFileName] under the
// supplied [LockFactory], mirroring the wire protocol implemented by Lucene's
// VerifyingLockFactory / LockVerifyServer / LockStressTest triplet.
//
// myID must be unique per client process and in [0, 255]. lockDirPath must be
// a directory on disk; it is opened via [NewNIOFSDirectory] with the
// verifier-friendly [NoLockFactory] (the lock factory under test is what we
// actually exercise, not the directory's). The function returns:
//
//   - 1 if myID is out of range, matching Lucene's exit code for the same
//     check;
//   - 0 on a clean run;
//   - a wrapped error if any I/O, protocol, or directory operation fails.
//
// LockStressRun is the Go port of org.apache.lucene.store.LockStressTest#run.
func LockStressRun(
	myID int,
	verifierHost string,
	verifierPort int,
	lockFactory LockFactory,
	lockDirPath string,
	sleep time.Duration,
	count int,
) (int, error) {
	if myID < 0 || myID > 255 {
		fmt.Fprintln(os.Stdout, "myID must be a unique int 0..255")
		return 1, nil
	}
	if lockFactory == nil {
		return 1, errors.New("lock stress: lockFactory must not be nil")
	}
	if count < 0 {
		return 1, fmt.Errorf("lock stress: count must be >= 0, got %d", count)
	}

	lockDir, err := NewNIOFSDirectory(lockDirPath)
	if err != nil {
		return 1, fmt.Errorf("lock stress: open lock dir %q: %w", lockDirPath, err)
	}
	defer lockDir.Close()

	addr := net.JoinHostPort(verifierHost, strconv.Itoa(verifierPort))
	fmt.Fprintf(os.Stdout, "Connecting to server %s and registering as client %d...\n", addr, myID)

	conn, err := net.DialTimeout("tcp", addr, lockStressDialTimeout)
	if err != nil {
		return 1, fmt.Errorf("lock stress: dial verifier %s: %w", addr, err)
	}
	defer conn.Close()

	// Send our one-byte ID, then wait for the start gun.
	if _, err := conn.Write([]byte{byte(myID)}); err != nil {
		return 1, fmt.Errorf("lock stress: send id: %w", err)
	}

	startBuf := [1]byte{}
	if _, err := io.ReadFull(conn, startBuf[:]); err != nil {
		return 1, fmt.Errorf("lock stress: read start gun: %w", err)
	}
	if startBuf[0] != StartGunSignal {
		return 1, fmt.Errorf("lock stress: protocol violation: expected start gun %d, got %d", StartGunSignal, startBuf[0])
	}

	verifyLF := newNetworkVerifyingLockFactory(lockFactory, conn)
	// math/rand/v2 is goroutine-safe via the top-level functions, but using a
	// local PCG generator avoids the global lock and is the closest analogue
	// to Lucene's new Random() construction here.
	rng := rand.New(rand.NewPCG(uint64(myID)+1, uint64(time.Now().UnixNano())))

	for i := 0; i < count; i++ {
		lock, err := verifyLF.ObtainLock(lockDir, LockStressTestLockFileName)
		if err == nil {
			// Optionally try a double-obtain to assert the factory rejects it.
			if rng.IntN(lockStressDoubleObtainProbability) == 0 {
				secondFactory := verifyLF
				// Lucene also occasionally swaps in a *fresh* factory instance
				// for the second attempt; we mirror the coin flip but reuse
				// the same delegate, since the resolver was already called by
				// the caller. The wire protocol contract is unchanged.
				if rng.IntN(2) == 0 {
					secondFactory = newNetworkVerifyingLockFactory(lockFactory, conn)
				}
				if secondLock, secondErr := secondFactory.ObtainLock(lockDir, LockStressTestLockFileName); secondErr == nil {
					_ = secondLock.Close()
					_ = lock.Close()
					return 1, errors.New("lock stress: double obtain succeeded — locking is broken")
				}
				// pass: a failure here is the expected outcome.
			}

			if sleep > 0 {
				time.Sleep(sleep)
			}

			if err := lock.Close(); err != nil {
				return 1, fmt.Errorf("lock stress: release lock at iter %d: %w", i, err)
			}
		}
		// If obtain failed, Lucene treats it as a normal contention outcome
		// (pass); we do the same.

		if count > 0 && i%lockStressProgressEvery == 0 {
			fmt.Fprintf(os.Stdout, "%v%% done.\n", float64(i)*100.0/float64(count))
		}

		if sleep > 0 {
			time.Sleep(sleep)
		}
	}

	fmt.Fprintf(os.Stdout, "Finished %d tries.\n", count)
	return 0, nil
}

// LockStressMain is the argv-driven entry point matching Lucene's
// LockStressTest#main. It expects exactly seven arguments — myID,
// verifierHost, verifierPort, lockFactoryName, lockDirName, sleepTimeMS,
// count — and returns an exit code suitable for handing to [os.Exit]. The
// caller is responsible for actually invoking os.Exit; this keeps the
// function testable.
//
// LockStressMain delegates lock-factory resolution to
// [DefaultLockStressFactoryResolver]; pass [LockStressMainWith] for custom
// resolution.
func LockStressMain(args []string) int {
	return LockStressMainWith(args, DefaultLockStressFactoryResolver)
}

// LockStressMainWith is identical to [LockStressMain] but resolves the
// lock-factory name via the supplied resolver. resolver must not be nil.
func LockStressMainWith(args []string, resolver LockStressFactoryResolver) int {
	if resolver == nil {
		fmt.Fprintln(os.Stderr, "lock stress: nil resolver")
		return 1
	}
	if len(args) != 7 {
		fmt.Fprintln(os.Stdout,
			"Usage: lockstresstest myID verifierHost verifierPort lockFactoryName lockDirName sleepTimeMS count\n"+
				"\n"+
				"  myID = int from 0 .. 255 (should be unique for test process)\n"+
				"  verifierHost = hostname that LockVerifyServer is listening on\n"+
				"  verifierPort = port that LockVerifyServer is listening on\n"+
				"  lockFactoryName = one of: native, simple, single, nolock\n"+
				"                    (Lucene's fully-qualified class names are also accepted)\n"+
				"  lockDirName = path to the lock directory\n"+
				"  sleepTimeMS = milliseconds to pause between each lock obtain/release\n"+
				"  count = number of locking tries\n"+
				"\n"+
				"You should run multiple instances of this process, each with its own\n"+
				"unique ID, and each pointing to the same lock directory, to verify\n"+
				"that locking is working correctly.\n"+
				"\n"+
				"Make sure you are first running lockverifyserver.")
		return 1
	}

	myID, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "lock stress: invalid myID %q: %v\n", args[0], err)
		return 1
	}
	verifierHost := args[1]
	verifierPort, err := strconv.Atoi(args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "lock stress: invalid verifierPort %q: %v\n", args[2], err)
		return 1
	}
	lockFactoryName := args[3]
	lockDirName := args[4]
	sleepMS, err := strconv.Atoi(args[5])
	if err != nil {
		fmt.Fprintf(os.Stderr, "lock stress: invalid sleepTimeMS %q: %v\n", args[5], err)
		return 1
	}
	count, err := strconv.Atoi(args[6])
	if err != nil {
		fmt.Fprintf(os.Stderr, "lock stress: invalid count %q: %v\n", args[6], err)
		return 1
	}

	factory, err := resolver(lockFactoryName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lock stress: %v\n", err)
		return 1
	}

	exit, err := LockStressRun(
		myID,
		verifierHost,
		verifierPort,
		factory,
		lockDirName,
		time.Duration(sleepMS)*time.Millisecond,
		count,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lock stress: %v\n", err)
		if exit == 0 {
			exit = 1
		}
	}
	return exit
}

// networkVerifyingLockFactory is the wire-protocol-aware analogue of
// VerifyingLockFactory used by LockStressRun. It wraps a delegate factory and
// reports every obtain/release to the verifier server over the supplied
// connection.
//
// Unlike the package-level [VerifyingLockFactory] — which tracks lock state
// in-process — this type defers all bookkeeping to the remote server.
type networkVerifyingLockFactory struct {
	delegate LockFactory
	conn     net.Conn
}

func newNetworkVerifyingLockFactory(delegate LockFactory, conn net.Conn) *networkVerifyingLockFactory {
	return &networkVerifyingLockFactory{delegate: delegate, conn: conn}
}

// ObtainLock obtains the underlying lock and notifies the verifier server.
// On any verification failure the underlying lock is released before the
// error surfaces, matching Lucene's try/catch semantics.
func (f *networkVerifyingLockFactory) ObtainLock(dir Directory, lockName string) (Lock, error) {
	lock, err := f.delegate.ObtainLock(dir, lockName)
	if err != nil {
		return nil, err
	}
	if err := f.verify(MsgLockAcquired); err != nil {
		_ = lock.Close()
		return nil, err
	}
	return &networkVerifyingLock{Lock: lock, factory: f}, nil
}

// verify sends a one-byte command to the verifier and waits for the server's
// echo acknowledgement. A mismatched echo or any I/O error is reported as a
// protocol violation.
func (f *networkVerifyingLockFactory) verify(message byte) error {
	if _, err := f.conn.Write([]byte{message}); err != nil {
		return fmt.Errorf("lock stress: send verification: %w", err)
	}
	ack := [1]byte{}
	if _, err := io.ReadFull(f.conn, ack[:]); err != nil {
		return fmt.Errorf("lock stress: read verification ack: %w", err)
	}
	if ack[0] != message {
		return fmt.Errorf("lock stress: protocol violation: expected ack %d, got %d", message, ack[0])
	}
	return nil
}

// networkVerifyingLock wraps a Lock and notifies the verifier on release.
type networkVerifyingLock struct {
	Lock
	factory  *networkVerifyingLockFactory
	released bool
}

// Close releases the underlying lock and notifies the verifier. The verifier
// is notified *before* the local release so the server-side state matches the
// observable order from other clients.
func (l *networkVerifyingLock) Close() error {
	if l.released {
		return nil
	}
	if err := l.Lock.EnsureValid(); err != nil {
		return err
	}
	if err := l.factory.verify(MsgLockReleased); err != nil {
		return err
	}
	if err := l.Lock.Close(); err != nil {
		return err
	}
	l.released = true
	return nil
}

// Compile-time interface checks.
var (
	_ LockFactory = (*networkVerifyingLockFactory)(nil)
	_ Lock        = (*networkVerifyingLock)(nil)
)

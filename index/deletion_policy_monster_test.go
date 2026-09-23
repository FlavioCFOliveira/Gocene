// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// The @Nightly test port of
// lucene/core/src/test/org/apache/lucene/index/TestDeletionPolicy.java
// (Apache Lucene 10.5.0), built only with the gocene_monsters tag as Lucene
// runs it only when nightly tests are enabled.

package index_test

import (
	"maps"
	"strconv"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// deletionPolicyExpirationTime is the inner ExpirationTimeDeletionPolicy: it
// deletes a commit only when it has been obsoleted by N seconds.
type deletionPolicyExpirationTime struct {
	dir                   store.Directory
	expirationTimeSeconds float64
	numDelete             int
}

func (p *deletionPolicyExpirationTime) OnInit(commits []index.Commit) error {
	if len(commits) == 0 {
		return nil
	}
	if err := deletionPolicyVerifyCommitOrder(commits); err != nil {
		return err
	}
	return p.OnCommit(commits)
}

func (p *deletionPolicyExpirationTime) OnCommit(commits []index.Commit) error {
	if err := deletionPolicyVerifyCommitOrder(commits); err != nil {
		return err
	}

	lastCommit := commits[len(commits)-1]

	// Any commit older than expireTime should be deleted:
	lastTime, err := getCommitTime(lastCommit)
	if err != nil {
		return err
	}
	expireTime := float64(lastTime)/1000.0 - p.expirationTimeSeconds

	for _, commit := range commits {
		commitTime, err := getCommitTime(commit)
		if err != nil {
			return err
		}
		modTime := float64(commitTime) / 1000.0
		if commit != lastCommit && modTime < expireTime {
			if err := commit.Delete(); err != nil {
				return err
			}
			p.numDelete++
		}
	}
	return nil
}

func (p *deletionPolicyExpirationTime) Clone() index.IndexDeletionPolicy { return p }

// Test "by time expiration" deletion policy:
// TODO: this wall-clock-dependent test doesn't seem to actually test any
// deletionpolicy logic?
func TestDeletionPolicyExpirationTimeDeletionPolicy(t *testing.T) {
	const seconds = 2.0

	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetIndexDeletionPolicy(&deletionPolicyExpirationTime{dir: dir, expirationTimeSeconds: seconds})
	setNoCFSRatio(conf, 1.0)
	writer := mustNewIndexWriter(t, dir, conf)
	policy := writer.GetConfig().GetIndexDeletionPolicy().(*deletionPolicyExpirationTime)
	commitData := map[string]string{"commitTime": strconv.FormatInt(time.Now().UnixNano(), 10)}
	writer.SetLiveCommitData(maps.All(commitData))
	mustCommit(t, writer)
	mustClose(t, writer)

	targetNumDelete := nextInt(1, 5)
	for policy.numDelete < targetNumDelete {
		// Record last time when writer performed deletes of
		// past commits
		conf = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		conf.SetOpenMode(index.Append)
		conf.SetIndexDeletionPolicy(policy)
		setNoCFSRatio(conf, 1.0)
		writer = mustNewIndexWriter(t, dir, conf)
		policy = writer.GetConfig().GetIndexDeletionPolicy().(*deletionPolicyExpirationTime)
		for j := 0; j < 17; j++ {
			deletionPolicyAddDoc(t, writer)
		}
		commitData = map[string]string{"commitTime": strconv.FormatInt(time.Now().UnixNano(), 10)}
		writer.SetLiveCommitData(maps.All(commitData))
		mustCommit(t, writer)
		mustClose(t, writer)

		time.Sleep(time.Duration(1000.0*(seconds/5.0)) * time.Millisecond)
	}

	// Then simplistic check: just verify that the
	// segments_N's that still exist are in fact within SECONDS
	// seconds of the last one's mod time, and, that I can
	// open a reader on each:
	mustClose(t, mustOpenDirectoryReader(t, dir))
	mustClose(t, dir)
	t.Fatal("org.apache.lucene.index.SegmentInfos#getCommitLuceneVersion() and " +
		"SegmentInfos#getMinSegmentLuceneVersion() are not ported")
}

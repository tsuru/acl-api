// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storagetest

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tsuru/acl-api/api/types"
	"github.com/tsuru/acl-api/storage"
)

type SyncStorageSuite struct {
	suite.Suite
	SetupTestFunc func()
	Stor          storage.SyncStorage
}

func (s *SyncStorageSuite) SetupTest() {
	s.SetupTestFunc()
}

func (s *SyncStorageSuite) TestStartEndSync() {
	t := s.T()
	lockTime := 500 * time.Millisecond
	_, rs1, err := s.Stor.StartSync(lockTime, "r1", "e1", false)
	require.Nil(t, err)
	require.NotEmpty(t, rs1.SyncID)
	_, _, err = s.Stor.StartSync(lockTime, "r1", "e1", false)
	require.Equal(t, storage.ErrSyncStorageLocked, err)
	_, rs2, err := s.Stor.StartSync(lockTime, "r1", "e2", false)
	require.Nil(t, err)
	require.NotEmpty(t, rs2.SyncID)
	require.NotEqual(t, rs1.SyncID, rs2.SyncID)
	time.Sleep(2 * lockTime)
	_, _, err = s.Stor.StartSync(lockTime, "r1", "e1", false)
	require.Equal(t, storage.ErrSyncStorageLocked, err)
	err = s.Stor.EndSync(*rs1, types.RuleSyncData{})
	require.Nil(t, err)
	err = s.Stor.EndSync(*rs1, types.RuleSyncData{})
	require.Nil(t, err)
	err = s.Stor.EndSync(*rs2, types.RuleSyncData{})
	require.Nil(t, err)
	_, _, err = s.Stor.StartSync(lockTime, "r1", "e1", false)
	require.Equal(t, storage.ErrSyncStorageLocked, err)
	time.Sleep(2 * lockTime)
	_, rs3, err := s.Stor.StartSync(lockTime, "r1", "e1", false)
	require.Nil(t, err)
	require.NotEmpty(t, rs3.SyncID)
	require.Equal(t, rs1.SyncID, rs3.SyncID)
}

func assertDuration(t *testing.T, expected, real time.Duration) {
	tolleration := 50 * time.Millisecond
	assert.True(t, math.Abs(float64(expected-real)) < float64(tolleration),
		"expected: %v+-%v, got: %v", expected, tolleration, real)
}

func (s *SyncStorageSuite) TestStartSyncNext() {
	t := s.T()
	lockTime := 500 * time.Millisecond
	next, rs, err := s.Stor.StartSync(lockTime, "r1", "e1", false)
	require.Nil(t, err)
	assertDuration(t, lockTime, next)
	next, _, err = s.Stor.StartSync(lockTime, "r1", "e1", false)
	require.Equal(t, storage.ErrSyncStorageLocked, err)
	assertDuration(t, lockTime, next)
	err = s.Stor.EndSync(*rs, types.RuleSyncData{})
	require.Nil(t, err)
	time.Sleep(lockTime / 2)
	next, _, err = s.Stor.StartSync(lockTime, "r1", "e1", false)
	require.Equal(t, storage.ErrSyncStorageLocked, err)
	assertDuration(t, lockTime/2, next)
	time.Sleep(next + 10*time.Millisecond)
	_, _, err = s.Stor.StartSync(lockTime, "r1", "e1", false)
	require.Nil(t, err)
}

func (s *SyncStorageSuite) TestStartExpireEndEnd() {
	t := s.T()
	defer s.Stor.SetLockExpireTime(s.Stor.SetLockExpireTime(700 * time.Millisecond))
	lockTime := 200 * time.Millisecond
	_, rs, err := s.Stor.StartSync(lockTime, "r1", "e1", false)
	require.Nil(t, err)
	_, _, err = s.Stor.StartSync(lockTime, "r1", "e1", false)
	require.Equal(t, storage.ErrSyncStorageLocked, err)
	time.Sleep(time.Second)
	_, _, err = s.Stor.StartSync(lockTime, "r1", "e1", false)
	require.Nil(t, err)
	err = s.Stor.EndSync(*rs, types.RuleSyncData{})
	require.Nil(t, err)
}

func (s *SyncStorageSuite) TestLockUnlockConcurrent() {
	lockTime := 500 * time.Millisecond
	t := s.T()
	n := 20
	var successful int32
	wg := sync.WaitGroup{}
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := s.Stor.StartSync(lockTime, "r1", "e1", false)
			if err == nil {
				atomic.AddInt32(&successful, 1)
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(1), successful)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.Stor.EndSync(types.RuleSyncInfo{RuleID: "r1", Engine: "e1"}, types.RuleSyncData{})
			require.Nil(t, err)
		}()
	}
	wg.Wait()
	time.Sleep(2 * lockTime)
	_, _, err := s.Stor.StartSync(lockTime, "r1", "e1", false)
	require.Nil(t, err)
}

func (s *SyncStorageSuite) TestAddSyncDataFind() {
	t := s.T()
	ts := time.Date(1984, 7, 10, 15, 0, 0, 0, time.UTC)
	_, ruleSync, err := s.Stor.StartSync(-time.Hour, "r1", "e1", false)
	require.Nil(t, err)
	err = s.Stor.EndSync(*ruleSync, types.RuleSyncData{
		StartTime:  ts,
		EndTime:    ts,
		Successful: true,
		SyncResult: "something",
	})
	require.Nil(t, err)
	_, ruleSync, err = s.Stor.StartSync(-time.Hour, "r1", "e1", false)
	require.Nil(t, err)
	err = s.Stor.EndSync(*ruleSync, types.RuleSyncData{
		StartTime:  ts,
		EndTime:    ts,
		Successful: true,
		SyncResult: "other",
	})
	require.Nil(t, err)
	_, ruleSync, err = s.Stor.StartSync(-time.Hour, "r2", "e1", false)
	require.Nil(t, err)
	err = s.Stor.EndSync(*ruleSync, types.RuleSyncData{
		StartTime:  ts,
		EndTime:    ts,
		Successful: true,
		SyncResult: "r2-other",
	})
	require.Nil(t, err)
	syncs, err := s.Stor.Find(storage.SyncFindOpts{})
	require.Nil(t, err)
	compareSyncs(t, []types.RuleSyncInfo{
		{
			RuleID:  "r1",
			Engine:  "e1",
			Running: false,
			Syncs: []types.RuleSyncData{
				{
					StartTime:  ts,
					EndTime:    ts,
					Successful: true,
					SyncResult: "something",
				},
				{
					StartTime:  ts,
					EndTime:    ts,
					Successful: true,
					SyncResult: "other",
				},
			},
		},
		{
			RuleID:  "r2",
			Engine:  "e1",
			Running: false,
			Syncs: []types.RuleSyncData{
				{
					StartTime:  ts,
					EndTime:    ts,
					Successful: true,
					SyncResult: "r2-other",
				},
			},
		},
	}, syncs)
	syncs, err = s.Stor.Find(storage.SyncFindOpts{
		RuleIDs: []string{"r1"},
	})
	require.Nil(t, err)
	compareSyncs(t, []types.RuleSyncInfo{
		{
			RuleID:  "r1",
			Engine:  "e1",
			Running: false,
			Syncs: []types.RuleSyncData{
				{
					StartTime:  ts,
					EndTime:    ts,
					Successful: true,
					SyncResult: "something",
				},
				{
					StartTime:  ts,
					EndTime:    ts,
					Successful: true,
					SyncResult: "other",
				},
			},
		},
	}, syncs)
	syncs, err = s.Stor.Find(storage.SyncFindOpts{
		RuleIDs: []string{},
	})
	require.Nil(t, err)
	compareSyncs(t, []types.RuleSyncInfo{}, syncs)
}

func compareSyncs(t *testing.T, expected, got []types.RuleSyncInfo) {
	assert.Equal(t, len(expected), len(got))
	sort.Slice(got, func(i, j int) bool {
		return got[i].RuleID < got[j].RuleID
	})
	for i := range got {
		got[i].StartTime = time.Time{}
		got[i].EndTime = time.Time{}
		got[i].PingTime = time.Time{}
		assert.NotEmpty(t, got[i].SyncID)
		got[i].SyncID = ""
	}
	assert.Equal(t, expected, got)
}

func (s *SyncStorageSuite) TestEndSyncOnlyLatest10Syncs() {
	t := s.T()
	for i := 0; i < 12; i++ {
		_, ruleSync, err := s.Stor.StartSync(-time.Hour, "r1", "e1", false)
		require.Nil(t, err)
		err = s.Stor.EndSync(*ruleSync, types.RuleSyncData{
			Successful: true,
			SyncResult: fmt.Sprintf("something-%d", i),
		})
		require.Nil(t, err)
	}
	ruleSyncs, err := s.Stor.Find(storage.SyncFindOpts{})
	require.Nil(t, err)
	require.Len(t, ruleSyncs, 1)
	require.Len(t, ruleSyncs[0].Syncs, 10)
	assert.Equal(t, "something-2", ruleSyncs[0].Syncs[0].SyncResult)
	assert.Equal(t, "something-11", ruleSyncs[0].Syncs[9].SyncResult)
}

// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rule

import (
	"math/rand"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/acl-api/api/types"
	"github.com/tsuru/acl-api/storage"
	_ "github.com/tsuru/acl-api/storage/mongodb"
)

func init() {
	viper.AutomaticEnv()
	storagePath := viper.GetString("storage")
	if storagePath == "" {
		storagePath = "mongodb://localhost"
	}
	viper.Set("storage", storagePath+"/acltest-pkg-rule")
}

func Test_RuleService_Save(t *testing.T) {
	stor, err := storage.GetRuleStorage()
	require.Nil(t, err)
	clearer := stor.(interface {
		ClearAll()
	})
	t.Run("ok", func(t *testing.T) {
		clearer.ClearAll()
		r := types.Rule{
			Source: types.RuleType{
				ExternalDNS: &types.ExternalDNSRule{
					Name: "x.com",
					Ports: []types.ProtoPort{
						{Protocol: "tcp", Port: 22},
					},
				},
			},
			Destination: types.RuleType{
				ExternalDNS: &types.ExternalDNSRule{
					Name: "x.com",
				},
			},
		}
		svc := GetService()
		err := svc.Save([]*types.Rule{&r}, false)
		require.Nil(t, err)
		assert.NotEmpty(t, r.RuleID)
		assert.NotEmpty(t, r.Created)
		rules, err := svc.FindAll()
		require.Nil(t, err)
		require.Len(t, rules, 1)
		assert.Equal(t, []types.Rule{
			{
				RuleID: rules[0].RuleID,
				Source: types.RuleType{
					ExternalDNS: &types.ExternalDNSRule{
						Name: "x.com",
						Ports: []types.ProtoPort{
							{Protocol: "tcp", Port: 22},
						},
					},
				},
				Destination: types.RuleType{
					ExternalDNS: &types.ExternalDNSRule{
						Name:  "x.com",
						Ports: []types.ProtoPort{},
					},
				},
				Metadata: map[string]string{},
				Created:  rules[0].Created,
			},
		}, rules)
	})
	t.Run("validations", func(t *testing.T) {
		clearer.ClearAll()
		tests := []struct {
			r   types.Rule
			err string
		}{
			{r: types.Rule{}, err: `source: exactly one rule type must be set`},
			{r: types.Rule{
				Source: types.RuleType{
					ExternalDNS: &types.ExternalDNSRule{
						Name: "x.com",
					},
				},
			}, err: `destination: exactly one rule type must be set`},
			{r: types.Rule{
				Source: types.RuleType{
					ExternalDNS: &types.ExternalDNSRule{
						Name: "x.com",
						Ports: []types.ProtoPort{
							{},
						},
					},
				},
				Destination: types.RuleType{
					ExternalDNS: &types.ExternalDNSRule{
						Name: "x.com",
					},
				},
			}, err: `source: invalid port number 0`},
			{r: types.Rule{
				Source: types.RuleType{
					ExternalDNS: &types.ExternalDNSRule{
						Name: "x.com",
						Ports: []types.ProtoPort{
							{Port: 21},
						},
					},
				},
				Destination: types.RuleType{
					ExternalDNS: &types.ExternalDNSRule{
						Name: "x.com",
					},
				},
			}, err: `source: invalid protocol "", valid values are: TCP, UDP`},
			{r: types.Rule{
				Source: types.RuleType{
					ExternalDNS: &types.ExternalDNSRule{
						Name: "x.com",
						Ports: []types.ProtoPort{
							{Port: 21, Protocol: "http"},
						},
					},
				},
				Destination: types.RuleType{
					ExternalDNS: &types.ExternalDNSRule{
						Name: "x.com",
					},
				},
			}, err: `source: invalid protocol "http", valid values are: TCP, UDP`},
			{r: types.Rule{
				Source: types.RuleType{
					ExternalDNS: &types.ExternalDNSRule{
						Name: "x.com",
					},
				},
				Destination: types.RuleType{
					ExternalDNS: &types.ExternalDNSRule{
						Name: "x.com",
					},
				},
			}},
			{r: types.Rule{
				Source: types.RuleType{
					ExternalDNS: &types.ExternalDNSRule{
						Name: "",
					},
				},
			}, err: `source: cannot have empty external dns name`},
			{r: types.Rule{
				Source: types.RuleType{
					ExternalIP: &types.ExternalIPRule{
						IP: "",
					},
				},
			}, err: `source: cannot have empty external ip address`},
			{r: types.Rule{
				Source: types.RuleType{
					TsuruApp: &types.TsuruAppRule{
						AppName:  "",
						PoolName: "",
					},
				},
			}, err: `source: cannot have empty tsuru app name and pool name`},
			{r: types.Rule{
				Source: types.RuleType{
					TsuruApp: &types.TsuruAppRule{
						AppName:  "myapp",
						PoolName: "p1",
					},
				},
			}, err: `source: cannot set both app name and pool name`},
			{r: types.Rule{
				Source: types.RuleType{
					KubernetesService: &types.KubernetesServiceRule{
						ServiceName: "",
					},
				},
			}, err: `source: Kubernetes Service Rule: has been deactivated for use, please use instead: App or RPaaS destinations`},
		}
		svc := GetService()
		for _, tt := range tests {
			err := svc.Save([]*types.Rule{&tt.r}, false)
			if tt.err == "" {
				assert.Nil(t, err)
			} else {
				require.NotNil(t, err)
				assert.Regexp(t, tt.err, err.Error())
			}
		}
	})
}

func Test_RuleService_Delete(t *testing.T) {
	stor, err := storage.GetRuleStorage()
	require.Nil(t, err)
	clearer := stor.(interface {
		ClearAll()
	})
	t.Run("ok", func(t *testing.T) {
		clearer.ClearAll()
		r := types.Rule{
			RuleID: "1",
			Source: types.RuleType{
				ExternalDNS: &types.ExternalDNSRule{
					Name: "x.com",
				},
			},
			Destination: types.RuleType{
				ExternalDNS: &types.ExternalDNSRule{
					Name: "x.com",
				},
			},
		}
		svc := GetService()
		err := svc.Save([]*types.Rule{&r}, false)
		require.Nil(t, err)
		err = svc.Delete("1")
		require.Nil(t, err)
		rules, err := svc.FindAll()
		require.Nil(t, err)
		require.Len(t, rules, 1)
		assert.Equal(t, []types.Rule{{
			Removed: true,
			RuleID:  "1",
			Source: types.RuleType{
				ExternalDNS: &types.ExternalDNSRule{
					Name:  "x.com",
					Ports: []types.ProtoPort{},
				},
			},
			Destination: types.RuleType{
				ExternalDNS: &types.ExternalDNSRule{
					Name:  "x.com",
					Ports: []types.ProtoPort{},
				},
			},
			Metadata: map[string]string{},
			Created:  rules[0].Created,
		}}, rules)
	})
	t.Run("not found", func(t *testing.T) {
		clearer.ClearAll()
		svc := GetService()
		err := svc.Delete("1")
		require.Equal(t, storage.ErrRuleNotFound, err)
	})
}

func Test_RuleService_DeleteMetadata(t *testing.T) {
	stor, err := storage.GetRuleStorage()
	require.Nil(t, err)
	clearer := stor.(interface {
		ClearAll()
	})
	t.Run("ok", func(t *testing.T) {
		clearer.ClearAll()
		r := types.Rule{
			RuleID: "1",
			Source: types.RuleType{
				ExternalDNS: &types.ExternalDNSRule{
					Name: "x.com",
				},
			},
			Destination: types.RuleType{
				ExternalDNS: &types.ExternalDNSRule{
					Name: "x.com",
				},
			},
			Metadata: map[string]string{
				"x": "y",
			},
		}
		svc := GetService()
		err := svc.Save([]*types.Rule{&r}, false)
		require.Nil(t, err)
		err = svc.DeleteMetadata(map[string]string{"x": "y"})
		require.Nil(t, err)
		rules, err := svc.FindAll()
		require.Nil(t, err)
		require.Len(t, rules, 1)
		assert.Equal(t, []types.Rule{{
			Removed: true,
			RuleID:  "1",
			Source: types.RuleType{
				ExternalDNS: &types.ExternalDNSRule{
					Name:  "x.com",
					Ports: []types.ProtoPort{},
				},
			},
			Destination: types.RuleType{
				ExternalDNS: &types.ExternalDNSRule{
					Name:  "x.com",
					Ports: []types.ProtoPort{},
				},
			},
			Metadata: map[string]string{
				"x": "y",
			},
			Created: rules[0].Created,
		}}, rules)
	})
	t.Run("not found", func(t *testing.T) {
		clearer.ClearAll()
		svc := GetService()
		err := svc.DeleteMetadata(map[string]string{"x": "y"})
		require.Equal(t, storage.ErrRuleNotFound, err)
	})
}

func clearRSI(t *testing.T, rsis []types.RuleSyncInfo) []types.RuleSyncInfo {
	for i, rsi := range rsis {
		assert.NotEmpty(t, rsi.StartTime)
		assert.NotEmpty(t, rsi.PingTime)
		assert.NotEmpty(t, rsi.EndTime)
		assert.NotEmpty(t, rsi.SyncID)
		rsis[i].StartTime = time.Time{}
		rsis[i].PingTime = time.Time{}
		rsis[i].EndTime = time.Time{}
		rsis[i].SyncID = ""
	}
	return rsis
}

func Test_RuleService_SyncStartList(t *testing.T) {
	stor, err := storage.GetRuleStorage()
	require.Nil(t, err)
	clearer := stor.(interface {
		ClearAll()
	})
	t.Run("all rules", func(t *testing.T) {
		clearer.ClearAll()
		svc := GetService()
		_, rsi, err := svc.SyncStart(-time.Hour, "r1", "e1", false)
		require.Nil(t, err)
		err = svc.SyncEnd(*rsi, types.RuleSyncData{Successful: true})
		require.Nil(t, err)
		_, rsi, err = svc.SyncStart(-time.Hour, "r2", "e1", false)
		require.Nil(t, err)
		err = svc.SyncEnd(*rsi, types.RuleSyncData{Successful: true})
		require.Nil(t, err)
		_, rsi, err = svc.SyncStart(-time.Hour, "r2", "e1", false)
		require.Nil(t, err)
		err = svc.SyncEnd(*rsi, types.RuleSyncData{Error: "xyz"})
		require.Nil(t, err)
		dbSyncs, err := svc.FindSyncs(nil)
		require.Nil(t, err)
		sort.Slice(dbSyncs, func(i, j int) bool {
			return dbSyncs[i].RuleID < dbSyncs[j].RuleID
		})
		assert.Equal(t, []types.RuleSyncInfo{
			{
				RuleID: "r1",
				Engine: "e1",
				Syncs: []types.RuleSyncData{
					{
						Successful: true,
					},
				},
			},
			{
				RuleID: "r2",
				Engine: "e1",
				Syncs: []types.RuleSyncData{
					{
						Successful: true,
					},
					{
						Error: "xyz",
					},
				},
			},
		}, clearRSI(t, dbSyncs))
	})
	t.Run("filter rules", func(t *testing.T) {
		clearer.ClearAll()
		svc := GetService()
		_, rsi, err := svc.SyncStart(-time.Hour, "r1", "e1", false)
		require.Nil(t, err)
		err = svc.SyncEnd(*rsi, types.RuleSyncData{Successful: true})
		require.Nil(t, err)
		_, rsi, err = svc.SyncStart(-time.Hour, "r2", "e1", false)
		require.Nil(t, err)
		err = svc.SyncEnd(*rsi, types.RuleSyncData{Successful: true})
		require.NoError(t, err)
		_, rsi, err = svc.SyncStart(-time.Hour, "r3", "e1", false)
		require.Nil(t, err)
		err = svc.SyncEnd(*rsi, types.RuleSyncData{Successful: true})
		require.Nil(t, err)
		dbSyncs, err := svc.FindSyncs([]string{"r2"})
		require.Nil(t, err)
		assert.Equal(t, []types.RuleSyncInfo{
			{
				RuleID: "r2",
				Engine: "e1",
				Syncs: []types.RuleSyncData{
					{
						Successful: true,
					},
				},
			},
		}, clearRSI(t, dbSyncs))
	})
	t.Run("filter empty rules", func(t *testing.T) {
		clearer.ClearAll()
		svc := GetService()
		_, rsi, err := svc.SyncStart(-time.Hour, "r1", "e1", false)
		require.Nil(t, err)
		err = svc.SyncEnd(*rsi, types.RuleSyncData{Successful: true})
		require.NoError(t, err)
		dbSyncs, err := svc.FindSyncs([]string{})
		require.Nil(t, err)
		assert.Equal(t, []types.RuleSyncInfo{}, clearRSI(t, dbSyncs))
	})
	t.Run("not found", func(t *testing.T) {
		clearer.ClearAll()
		svc := GetService()
		dbSyncs, err := svc.FindSyncs(nil)
		require.Nil(t, err)
		assert.Equal(t, []types.RuleSyncInfo{}, dbSyncs)
	})
}

func Test_RuleService_SyncStartLock(t *testing.T) {
	stor, err := storage.GetRuleStorage()
	require.Nil(t, err)
	clearer := stor.(interface {
		ClearAll()
	})
	t.Run("lock and unlock", func(t *testing.T) {
		clearer.ClearAll()
		lockTime := 500 * time.Millisecond
		svc := GetService()
		_, rsi, err := svc.SyncStart(lockTime, "r1", "e1", false)
		require.Nil(t, err)
		defer svc.SyncEnd(*rsi, types.RuleSyncData{})
		_, _, err = svc.SyncStart(lockTime, "r1", "e1", false)
		require.Equal(t, storage.ErrSyncStorageLocked, err)
		time.Sleep(2 * lockTime)
		_, _, err = svc.SyncStart(lockTime, "r1", "e1", false)
		require.Equal(t, storage.ErrSyncStorageLocked, err)
		err = svc.SyncEnd(*rsi, types.RuleSyncData{})
		require.Nil(t, err)
		_, _, err = svc.SyncStart(lockTime, "r1", "e1", false)
		require.Equal(t, storage.ErrSyncStorageLocked, err)
		time.Sleep(2 * lockTime)
		_, _, err = svc.SyncStart(lockTime, "r1", "e1", false)
		require.Nil(t, err)
		err = svc.SyncEnd(*rsi, types.RuleSyncData{})
		require.Nil(t, err)
	})
	t.Run("lock and unlock updater", func(t *testing.T) {
		clearer.ClearAll()
		stor, err := storage.GetSyncStorage()
		require.Nil(t, err)
		updater.stop()
		oldInterval := lockUpdaterInterval
		lockUpdaterInterval = 100 * time.Millisecond
		updater.run()
		defer func() {
			updater.stop()
			lockUpdaterInterval = oldInterval
			updater.run()
		}()
		defer stor.SetLockExpireTime(stor.SetLockExpireTime(700 * time.Millisecond))
		lockTime := 200 * time.Millisecond
		svc := GetService()
		_, rsi, err := svc.SyncStart(lockTime, "r1", "e1", false)
		require.Nil(t, err)
		defer svc.SyncEnd(*rsi, types.RuleSyncData{})
		_, _, err = svc.SyncStart(lockTime, "r1", "e1", false)
		require.Equal(t, storage.ErrSyncStorageLocked, err)
		time.Sleep(time.Second)
		_, _, err = svc.SyncStart(lockTime, "r1", "e1", false)
		require.Equal(t, storage.ErrSyncStorageLocked, err)
		err = svc.SyncEnd(*rsi, types.RuleSyncData{})
		require.Nil(t, err)
		updater.stop()
		assert.Len(t, updater.syncIDSet, 0)
		updater.run()
	})
	t.Run("lock unlock concurrent stress", func(t *testing.T) {
		clearer.ClearAll()
		stor, err := storage.GetSyncStorage()
		require.Nil(t, err)
		updater.stop()
		oldInterval := lockUpdaterInterval
		lockUpdaterInterval = 50 * time.Millisecond
		updater.run()
		defer func() {
			updater.stop()
			lockUpdaterInterval = oldInterval
			updater.run()
		}()
		defer stor.SetLockExpireTime(stor.SetLockExpireTime(700 * time.Millisecond))
		lockTime := 100 * time.Millisecond
		svc := GetService()
		nGoroutines := 10
		wg := sync.WaitGroup{}
		for i := 0; i < nGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
				_, rsi, syncErr := svc.SyncStart(lockTime, "r1", "e1", false)
				if syncErr == nil {
					syncErr = svc.SyncEnd(*rsi, types.RuleSyncData{})
					require.Nil(t, syncErr)
				} else {
					assert.Equal(t, storage.ErrSyncStorageLocked, syncErr)
				}
			}()
		}
		wg.Wait()
		updater.stop()
		assert.Len(t, updater.syncIDSet, 0)
		updater.run()
		time.Sleep(2 * lockTime)
		_, rsi, err := svc.SyncStart(lockTime, "r1", "e1", false)
		require.Nil(t, err)
		err = svc.SyncEnd(*rsi, types.RuleSyncData{})
		require.Nil(t, err)
	})
}

func Test_RuleService_FindMetadata(t *testing.T) {
	stor, err := storage.GetRuleStorage()
	require.Nil(t, err)
	clearer := stor.(interface {
		ClearAll()
	})
	t.Run("ok", func(t *testing.T) {
		clearer.ClearAll()
		r1 := types.Rule{
			RuleID: "1",
			Source: types.RuleType{
				ExternalDNS: &types.ExternalDNSRule{
					Name: "x.com",
				},
			},
			Destination: types.RuleType{
				ExternalDNS: &types.ExternalDNSRule{
					Name: "x.com",
				},
			},
			Metadata: map[string]string{
				"x": "y",
			},
		}
		r2 := types.Rule{
			RuleID: "2",
			Source: types.RuleType{
				ExternalDNS: &types.ExternalDNSRule{
					Name: "x.com",
				},
			},
			Destination: types.RuleType{
				ExternalDNS: &types.ExternalDNSRule{
					Name: "x.com",
				},
			},
			Metadata: map[string]string{
				"x": "z",
			},
		}
		svc := GetService()
		err := svc.Save([]*types.Rule{&r1}, false)
		require.Nil(t, err)
		err = svc.Save([]*types.Rule{&r2}, false)
		require.Nil(t, err)
		rules, err := svc.FindMetadata(map[string]string{"x": "y"})
		require.Nil(t, err)
		require.Len(t, rules, 1)
		assert.Equal(t, []types.Rule{{
			RuleID: "1",
			Source: types.RuleType{
				ExternalDNS: &types.ExternalDNSRule{
					Name:  "x.com",
					Ports: []types.ProtoPort{},
				},
			},
			Destination: types.RuleType{
				ExternalDNS: &types.ExternalDNSRule{
					Name:  "x.com",
					Ports: []types.ProtoPort{},
				},
			},
			Metadata: map[string]string{
				"x": "y",
			},
			Created: rules[0].Created,
		}}, rules)
		rules, err = svc.FindMetadata(map[string]string{"x": "a"})
		require.Nil(t, err)
		require.Len(t, rules, 0)
	})
	t.Run("not found", func(t *testing.T) {
		clearer.ClearAll()
		svc := GetService()
		rules, err := svc.FindMetadata(map[string]string{"x": "y"})
		require.Nil(t, err)
		require.Len(t, rules, 0)
	})
}

func Test_RuleService_FindByRule(t *testing.T) {
	stor, err := storage.GetRuleStorage()
	require.Nil(t, err)
	clearer := stor.(interface {
		ClearAll()
	})
	clearer.ClearAll()
	rules := []types.Rule{
		{
			RuleID: "1",
			Source: types.RuleType{
				TsuruApp: &types.TsuruAppRule{
					AppName: "myapp",
				},
			},
			Destination: types.RuleType{
				ExternalDNS: &types.ExternalDNSRule{
					Name:  "x.com",
					Ports: []types.ProtoPort{},
				},
			},
			Metadata: map[string]string{
				"x": "y",
			},
		},
		{
			RuleID: "2",
			Source: types.RuleType{
				ExternalDNS: &types.ExternalDNSRule{
					Name:  "x.com",
					Ports: []types.ProtoPort{},
				},
			},
			Destination: types.RuleType{
				ExternalDNS: &types.ExternalDNSRule{
					Name:  "x.com",
					Ports: []types.ProtoPort{},
				},
			},
			Metadata: map[string]string{
				"x": "y",
			},
		},
	}
	svc := GetService()
	for _, r := range rules {
		err := svc.Save([]*types.Rule{&r}, false)
		require.Nil(t, err)
	}

	tests := []struct {
		filter          types.Rule
		expectedRuleIDs []string
	}{
		{
			filter:          types.Rule{},
			expectedRuleIDs: []string{"1", "2", "3", "4"},
		},
		{
			filter: types.Rule{
				Source: types.RuleType{
					TsuruApp: &types.TsuruAppRule{
						AppName: "myapp",
					},
				},
				Metadata: map[string]string{
					"x": "y",
				},
			},
			expectedRuleIDs: []string{"1"},
		},
		{
			filter: types.Rule{
				Metadata: map[string]string{
					"x": "y",
				},
			},
			expectedRuleIDs: []string{"1", "2"},
		},
		{
			filter: types.Rule{
				Metadata: map[string]string{
					"invalid": "y",
				},
			},
			expectedRuleIDs: []string{},
		},
		{
			filter: types.Rule{
				Source: types.RuleType{
					TsuruApp: &types.TsuruAppRule{},
				},
			},
			expectedRuleIDs: []string{"1"},
		},
		{
			filter: types.Rule{
				Destination: types.RuleType{
					TsuruApp: &types.TsuruAppRule{},
				},
			},
			expectedRuleIDs: []string{},
		},
		{
			filter: types.Rule{
				Source: types.RuleType{
					ExternalDNS: &types.ExternalDNSRule{
						Name: "x.com",
					},
				},
			},
			expectedRuleIDs: []string{"2"},
		},
		{
			filter: types.Rule{
				Creator: "user9",
			},
			expectedRuleIDs: []string{"3"},
		},
		{
			filter: types.Rule{
				Creator: "user-invalid",
			},
			expectedRuleIDs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			foundRules, err := svc.FindByRule(tt.filter)
			require.Nil(t, err)
			for i := range foundRules {
				foundRules[i].Created = time.Time{}
			}
			var expectedRules []types.Rule
			for _, id := range tt.expectedRuleIDs {
				for _, rule := range rules {
					if id == rule.RuleID {
						expectedRules = append(expectedRules, rule)
					}
				}
			}
			assert.Equal(t, expectedRules, foundRules)
		})
	}

	t.Run("no rules registered", func(t *testing.T) {
		clearer.ClearAll()
		svc := GetService()
		rules, err := svc.FindByRule(types.Rule{})
		require.Nil(t, err)
		require.Len(t, rules, 0)
	})
}

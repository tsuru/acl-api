// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package service

import (
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/acl-api/api/types"
	"github.com/tsuru/acl-api/rule"
	"github.com/tsuru/acl-api/storage"
	_ "github.com/tsuru/acl-api/storage/mongodb"
)

func init() {
	viper.AutomaticEnv()
	storagePath := viper.GetString("storage")
	if storagePath == "" {
		storagePath = "mongodb://localhost"
	}
	viper.Set("storage", storagePath+"/acltest-pkg-service")
}

func Test_Service_Create(t *testing.T) {
	stor, err := storage.GetServiceStorage()
	require.Nil(t, err)
	clearer := stor.(interface {
		ClearAll()
	})
	t.Run("ok", func(t *testing.T) {
		clearer.ClearAll()
		si := types.ServiceInstance{
			InstanceName: "x",
		}
		svc := GetService()
		err := svc.Create(si)
		require.Nil(t, err)
	})
	t.Run("existing", func(t *testing.T) {
		clearer.ClearAll()
		si := types.ServiceInstance{
			InstanceName: "x",
		}
		svc := GetService()
		err := svc.Create(si)
		require.Nil(t, err)
		err = svc.Create(si)
		require.Equal(t, storage.ErrInstanceAlreadyExists, err)
	})
}

func Test_Service_Find(t *testing.T) {
	stor, err := storage.GetServiceStorage()
	require.Nil(t, err)
	clearer := stor.(interface {
		ClearAll()
	})
	t.Run("ok", func(t *testing.T) {
		clearer.ClearAll()
		si := types.ServiceInstance{
			InstanceName: "x",
		}
		svc := GetService()
		err := svc.Create(si)
		require.Nil(t, err)
		dbSi, err := svc.Find("x")
		require.Nil(t, err)
		assert.Equal(t, types.ServiceInstance{
			InstanceName: "x",
			BindApps:     []string{},
			BindJobs:     []string{},
			BaseRules:    []types.ServiceRule{},
		}, dbSi)
	})
	t.Run("not found", func(t *testing.T) {
		clearer.ClearAll()
		svc := GetService()
		_, err := svc.Find("x")
		require.Equal(t, storage.ErrInstanceNotFound, err)
	})
}

func Test_Service_Delete(t *testing.T) {
	stor, err := storage.GetServiceStorage()
	require.Nil(t, err)
	clearer := stor.(interface {
		ClearAll()
	})
	t.Run("ok", func(t *testing.T) {
		clearer.ClearAll()
		si := types.ServiceInstance{
			InstanceName: "x",
		}
		svc := GetService()
		err := svc.Create(si)
		require.Nil(t, err)
		err = svc.Delete("x")
		require.Nil(t, err)
	})
	t.Run("not found", func(t *testing.T) {
		clearer.ClearAll()
		svc := GetService()
		err := svc.Delete("x")
		require.Equal(t, storage.ErrInstanceNotFound, err)
	})
	t.Run("remove rules", func(t *testing.T) {
		clearer.ClearAll()
		si := types.ServiceInstance{
			InstanceName: "x",
		}
		svc := GetService()
		err := svc.Create(si)
		require.Nil(t, err)
		_, err = svc.AddApp("x", "app1")
		require.Nil(t, err)
		_, err = svc.AddRule("x", &types.ServiceRule{
			Rule: types.Rule{
				Destination: types.RuleType{
					TsuruApp: &types.TsuruAppRule{
						AppName: "app2",
					},
				},
			},
		})
		require.Nil(t, err)
		ruleSvc := rule.GetService()
		rules, err := ruleSvc.FindAll()
		require.Nil(t, err)
		assert.Len(t, rules, 1)
		assert.False(t, rules[0].Removed)
		err = svc.Delete("x")
		require.Nil(t, err)
		rules, err = ruleSvc.FindAll()
		require.Nil(t, err)
		assert.Len(t, rules, 1)
		assert.True(t, rules[0].Removed)
	})
}

func Test_Service_AddRule(t *testing.T) {
	stor, err := storage.GetServiceStorage()
	require.Nil(t, err)
	clearer := stor.(interface {
		ClearAll()
	})
	t.Run("ok", func(t *testing.T) {
		clearer.ClearAll()
		si := types.ServiceInstance{
			InstanceName: "x",
		}
		svc := GetService()
		err := svc.Create(si)
		require.Nil(t, err)
		syncedRules, err := svc.AddRule("x", &types.ServiceRule{
			Rule: types.Rule{
				Destination: types.RuleType{
					TsuruApp: &types.TsuruAppRule{
						AppName: "app2",
					},
				},
			},
		})
		require.Nil(t, err)
		assert.Nil(t, syncedRules)
		dbSi, err := svc.Find("x")
		require.Nil(t, err)
		require.Len(t, dbSi.BaseRules, 1)
		baseRuleID := dbSi.BaseRules[0].RuleID
		assert.NotEmpty(t, baseRuleID)
		assert.Equal(t, types.ServiceInstance{
			InstanceName: "x",
			BaseRules: []types.ServiceRule{
				{
					Rule: types.Rule{
						RuleID: baseRuleID,
						Destination: types.RuleType{
							TsuruApp: &types.TsuruAppRule{
								AppName: "app2",
							},
						},
						Metadata: map[string]string{},
						Created:  dbSi.BaseRules[0].Created,
					},
				},
			},
			BindApps: []string{},
			BindJobs: []string{},
		}, dbSi)
		syncedRules, err = svc.AddApp("x", "app1")
		require.Nil(t, err)
		expectedRules := []types.Rule{
			{
				RuleID: baseRuleID + "-app1",
				Source: types.RuleType{
					TsuruApp: &types.TsuruAppRule{
						AppName: "app1",
					},
				},
				Destination: types.RuleType{
					TsuruApp: &types.TsuruAppRule{
						AppName: "app2",
					},
				},
				Metadata: map[string]string{
					"owner":         "aclfromhell",
					"instance-name": "x",
					"app-name":      "app1",
					"base-ruleid":   baseRuleID,
				},
			},
		}
		compareRules(t, expectedRules, syncedRules)

		ruleSvc := rule.GetService()
		rules, err := ruleSvc.FindAll()
		require.Nil(t, err)
		compareRules(t, expectedRules, rules)
	})

	t.Run("invalid rule with bind", func(t *testing.T) {
		clearer.ClearAll()
		si := types.ServiceInstance{
			InstanceName: "x",
		}
		svc := GetService()
		err := svc.Create(si)
		require.NoError(t, err)
		syncedRules, err := svc.AddApp("x", "app1")
		require.NoError(t, err)
		assert.Nil(t, syncedRules)
		syncedRules, err = svc.AddRule("x", &types.ServiceRule{
			Rule: types.Rule{
				Destination: types.RuleType{
					ExternalDNS: &types.ExternalDNSRule{
						Name: "a.globo.com",
						Ports: []types.ProtoPort{
							{Protocol: "invalid", Port: 80},
						},
					},
				},
			},
		})
		require.EqualError(t, err, `invalid protocol "invalid", valid values are: TCP, UDP`)
		assert.Nil(t, syncedRules)
	})

	t.Run("invalid rule no binds", func(t *testing.T) {
		clearer.ClearAll()
		si := types.ServiceInstance{
			InstanceName: "x",
		}
		svc := GetService()
		err := svc.Create(si)
		require.NoError(t, err)
		syncedRules, err := svc.AddRule("x", &types.ServiceRule{
			Rule: types.Rule{
				Destination: types.RuleType{
					ExternalDNS: &types.ExternalDNSRule{
						Name: "a.globo.com",
						Ports: []types.ProtoPort{
							{Protocol: "invalid", Port: 80},
						},
					},
				},
			},
		})
		require.EqualError(t, err, `invalid protocol "invalid", valid values are: TCP, UDP`)
		assert.Nil(t, syncedRules)
	})
}

func Test_Service_RemoveRule(t *testing.T) {
	stor, err := storage.GetServiceStorage()
	require.Nil(t, err)
	clearer := stor.(interface {
		ClearAll()
	})
	t.Run("ok", func(t *testing.T) {
		clearer.ClearAll()
		si := types.ServiceInstance{
			InstanceName: "x",
		}
		svc := GetService()
		err := svc.Create(si)
		require.Nil(t, err)
		_, err = svc.AddRule("x", &types.ServiceRule{
			Rule: types.Rule{
				Destination: types.RuleType{
					TsuruApp: &types.TsuruAppRule{
						AppName: "app2",
					},
				},
			},
		})
		require.Nil(t, err)
		dbSi, err := svc.Find("x")
		require.Nil(t, err)
		baseRuleID := dbSi.BaseRules[0].RuleID
		assert.NotEmpty(t, baseRuleID)
		_, err = svc.AddApp("x", "app1")
		require.Nil(t, err)
		err = svc.RemoveRule("x", baseRuleID)
		require.Nil(t, err)

		ruleSvc := rule.GetService()
		rules, err := ruleSvc.FindAll()
		require.Nil(t, err)
		assert.Len(t, rules, 1)
		assert.True(t, rules[0].Removed)
	})
}

func Test_Service_AddApp(t *testing.T) {
	stor, err := storage.GetServiceStorage()
	require.Nil(t, err)
	clearer := stor.(interface {
		ClearAll()
	})
	t.Run("ok", func(t *testing.T) {
		clearer.ClearAll()
		si := types.ServiceInstance{
			InstanceName: "x",
		}
		svc := GetService()
		err := svc.Create(si)
		require.Nil(t, err)
		syncedRules, err := svc.AddApp("x", "app1")
		require.Nil(t, err)
		assert.Nil(t, syncedRules)
		dbSi, err := svc.Find("x")
		require.Nil(t, err)
		assert.Equal(t, types.ServiceInstance{
			InstanceName: "x",
			BaseRules:    []types.ServiceRule{},
			BindJobs:     []string{},
			BindApps:     []string{"app1"},
		}, dbSi)
		syncedRules, err = svc.AddRule("x", &types.ServiceRule{
			Rule: types.Rule{
				Destination: types.RuleType{
					TsuruApp: &types.TsuruAppRule{
						AppName: "app2",
					},
				},
			},
		})
		require.Nil(t, err)
		dbSi, err = svc.Find("x")
		require.Nil(t, err)
		baseRuleID := dbSi.BaseRules[0].RuleID
		assert.NotEmpty(t, baseRuleID)

		expectedRules := []types.Rule{
			{
				RuleID: baseRuleID + "-app1",
				Source: types.RuleType{
					TsuruApp: &types.TsuruAppRule{
						AppName: "app1",
					},
				},
				Destination: types.RuleType{
					TsuruApp: &types.TsuruAppRule{
						AppName: "app2",
					},
				},
				Metadata: map[string]string{
					"owner":         "aclfromhell",
					"instance-name": "x",
					"app-name":      "app1",
					"base-ruleid":   baseRuleID,
				},
			},
		}
		compareRules(t, expectedRules, syncedRules)

		ruleSvc := rule.GetService()
		rules, err := ruleSvc.FindAll()
		require.Nil(t, err)
		compareRules(t, expectedRules, rules)
	})
}

func Test_Service_AddJob(t *testing.T) {
	stor, err := storage.GetServiceStorage()
	require.Nil(t, err)
	clearer := stor.(interface {
		ClearAll()
	})
	t.Run("ok", func(t *testing.T) {
		clearer.ClearAll()
		si := types.ServiceInstance{
			InstanceName: "x",
		}
		svc := GetService()
		err := svc.Create(si)
		require.Nil(t, err)
		syncedRules, err := svc.AddJob("x", "job1")
		require.Nil(t, err)
		assert.Nil(t, syncedRules)
		dbSi, err := svc.Find("x")
		require.Nil(t, err)
		assert.Equal(t, types.ServiceInstance{
			InstanceName: "x",
			BaseRules:    []types.ServiceRule{},
			BindApps:     []string{},
			BindJobs:     []string{"job1"},
		}, dbSi)
		syncedRules, err = svc.AddRule("x", &types.ServiceRule{
			Rule: types.Rule{
				Destination: types.RuleType{
					TsuruApp: &types.TsuruAppRule{
						AppName: "app2",
					},
				},
			},
		})
		require.Nil(t, err)
		dbSi, err = svc.Find("x")
		require.Nil(t, err)
		baseRuleID := dbSi.BaseRules[0].RuleID
		assert.NotEmpty(t, baseRuleID)

		expectedRules := []types.Rule{
			{
				RuleID: "job-" + baseRuleID + "-job1",
				Source: types.RuleType{
					TsuruJob: &types.TsuruJobRule{
						JobName: "job1",
					},
				},
				Destination: types.RuleType{
					TsuruApp: &types.TsuruAppRule{
						AppName: "app2",
					},
				},
				Metadata: map[string]string{
					"owner":         "aclfromhell",
					"instance-name": "x",
					"job-name":      "job1",
					"base-ruleid":   baseRuleID,
				},
			},
		}
		compareRules(t, expectedRules, syncedRules)

		ruleSvc := rule.GetService()
		rules, err := ruleSvc.FindAll()
		require.Nil(t, err)
		compareRules(t, expectedRules, rules)
	})
}

func Test_Service_RemoveApp(t *testing.T) {
	stor, err := storage.GetServiceStorage()
	require.Nil(t, err)
	clearer := stor.(interface {
		ClearAll()
	})
	t.Run("ok", func(t *testing.T) {
		clearer.ClearAll()
		si := types.ServiceInstance{
			InstanceName: "x",
		}
		svc := GetService()
		err := svc.Create(si)
		require.Nil(t, err)
		_, err = svc.AddApp("x", "app1")
		require.Nil(t, err)
		_, err = svc.AddRule("x", &types.ServiceRule{
			Rule: types.Rule{
				Destination: types.RuleType{
					TsuruApp: &types.TsuruAppRule{
						AppName: "app2",
					},
				},
			},
		})
		require.Nil(t, err)

		err = svc.RemoveApp("x", "app1")
		require.Nil(t, err)

		ruleSvc := rule.GetService()
		rules, err := ruleSvc.FindAll()
		require.Nil(t, err)
		assert.Len(t, rules, 1)
		assert.True(t, rules[0].Removed)
	})
}

func Test_Service_RemoveJob(t *testing.T) {
	stor, err := storage.GetServiceStorage()
	require.Nil(t, err)
	clearer := stor.(interface {
		ClearAll()
	})
	t.Run("ok", func(t *testing.T) {
		clearer.ClearAll()
		si := types.ServiceInstance{
			InstanceName: "x",
		}
		svc := GetService()
		err := svc.Create(si)
		require.Nil(t, err)
		_, err = svc.AddJob("x", "job1")
		require.Nil(t, err)
		_, err = svc.AddRule("x", &types.ServiceRule{
			Rule: types.Rule{
				Destination: types.RuleType{
					TsuruApp: &types.TsuruAppRule{
						AppName: "app2",
					},
				},
			},
		})
		require.Nil(t, err)

		err = svc.RemoveJob("x", "job1")
		require.Nil(t, err)

		ruleSvc := rule.GetService()
		rules, err := ruleSvc.FindAll()
		require.Nil(t, err)
		assert.Len(t, rules, 1)
		assert.True(t, rules[0].Removed)
	})
}

func compareRules(t *testing.T, expected, got []types.Rule) {
	for i := range got {
		assert.NotEqual(t, got[i].Created, time.Time{})
		got[i].Created = time.Time{}
	}
	assert.Equal(t, expected, got)
}

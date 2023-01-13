// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storagetest

import (
	"sort"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tsuru/acl-api/api/types"
	"github.com/tsuru/acl-api/storage"
)

type ServiceStorageSuite struct {
	suite.Suite
	SetupTestFunc func()
	Stor          storage.ServiceStorage
}

func (s *ServiceStorageSuite) SetupTest() {
	s.SetupTestFunc()
}

func (s *ServiceStorageSuite) TestSave() {
	t := s.T()
	si := types.ServiceInstance{
		InstanceName: "inst1",
	}
	err := s.Stor.Create(si)
	require.Nil(t, err)
	dbSi, err := s.Stor.Find("inst1")
	require.Nil(t, err)
	assert.Equal(t, types.ServiceInstance{
		InstanceName: "inst1",
		BindApps:     []string{},
		BaseRules:    []types.ServiceRule{},
	}, dbSi)
	err = s.Stor.Create(si)
	assert.Equal(t, storage.ErrInstanceAlreadyExists, err)
}

func (s *ServiceStorageSuite) TestDelete() {
	t := s.T()
	si := types.ServiceInstance{
		InstanceName: "inst1",
	}
	err := s.Stor.Create(si)
	require.Nil(t, err)
	err = s.Stor.Delete("inst1")
	require.Nil(t, err)
	_, err = s.Stor.Find("inst1")
	assert.Equal(t, storage.ErrInstanceNotFound, err)
	err = s.Stor.Delete("inst1")
	assert.Equal(t, storage.ErrInstanceNotFound, err)
}

func (s *ServiceStorageSuite) TestAddRule() {
	t := s.T()
	si := types.ServiceInstance{
		InstanceName: "inst1",
	}
	err := s.Stor.Create(si)
	require.Nil(t, err)
	r := &types.ServiceRule{}
	err = s.Stor.AddRule("inst1", r)
	require.Nil(t, err)
	dbSi, err := s.Stor.Find("inst1")
	require.NoError(t, err)
	assert.Len(t, dbSi.BaseRules, 1)
	assert.NotEmpty(t, dbSi.BaseRules[0].RuleID)
	assert.Equal(t, types.ServiceInstance{
		InstanceName: "inst1",
		BindApps:     []string{},
		BaseRules: []types.ServiceRule{
			{Rule: types.Rule{RuleID: dbSi.BaseRules[0].RuleID, Metadata: map[string]string{}, Created: dbSi.BaseRules[0].Created}},
		},
	}, dbSi)
}

func (s *ServiceStorageSuite) TestRemoveRule() {
	t := s.T()
	si := types.ServiceInstance{
		InstanceName: "inst1",
	}
	err := s.Stor.Create(si)
	require.Nil(t, err)
	err = s.Stor.AddRule("inst1", &types.ServiceRule{Rule: types.Rule{RuleID: "rule1"}})
	require.Nil(t, err)
	err = s.Stor.AddRule("inst1", &types.ServiceRule{Rule: types.Rule{RuleID: "rule2"}})
	require.Nil(t, err)
	err = s.Stor.RemoveRule("inst1", "rule1")
	require.Nil(t, err)
	dbSi, err := s.Stor.Find("inst1")
	require.NoError(t, err)
	assert.Equal(t, types.ServiceInstance{
		InstanceName: "inst1",
		BindApps:     []string{},
		BaseRules: []types.ServiceRule{
			{Rule: types.Rule{RuleID: "rule2", Metadata: map[string]string{}, Created: dbSi.BaseRules[0].Created}},
		},
	}, dbSi)
}

func (s *ServiceStorageSuite) TestAddApp() {
	t := s.T()
	si := types.ServiceInstance{
		InstanceName: "inst1",
	}
	err := s.Stor.Create(si)
	require.Nil(t, err)
	err = s.Stor.AddApp("inst1", "app1")
	require.Nil(t, err)
	err = s.Stor.AddApp("inst1", "app2")
	require.Nil(t, err)
	err = s.Stor.AddApp("inst1", "app1")
	require.Nil(t, err)
	dbSi, err := s.Stor.Find("inst1")
	require.NoError(t, err)
	sort.Strings(dbSi.BindApps)
	assert.Equal(t, types.ServiceInstance{
		InstanceName: "inst1",
		BindApps:     []string{"app1", "app2"},
		BaseRules:    []types.ServiceRule{},
	}, dbSi)
}

func (s *ServiceStorageSuite) TestRemoveApp() {
	t := s.T()
	si := types.ServiceInstance{
		InstanceName: "inst1",
	}
	err := s.Stor.Create(si)
	require.Nil(t, err)
	err = s.Stor.AddApp("inst1", "app1")
	require.Nil(t, err)
	err = s.Stor.AddApp("inst1", "app2")
	require.Nil(t, err)
	err = s.Stor.RemoveApp("inst1", "app1")
	require.Nil(t, err)
	dbSi, err := s.Stor.Find("inst1")
	require.NoError(t, err)
	assert.Equal(t, types.ServiceInstance{
		InstanceName: "inst1",
		BindApps:     []string{"app2"},
		BaseRules:    []types.ServiceRule{},
	}, dbSi)
}

func (s *ServiceStorageSuite) TestList() {
	t := s.T()
	si := types.ServiceInstance{
		InstanceName: "inst1",
	}
	err := s.Stor.Create(si)
	require.Nil(t, err)
	dbSi, err := s.Stor.List()
	require.Nil(t, err)
	assert.Equal(t, []types.ServiceInstance{
		{
			InstanceName: "inst1",
			BindApps:     []string{},
			BaseRules:    []types.ServiceRule{},
		},
	}, dbSi)
}

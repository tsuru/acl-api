// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storagetest

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tsuru/acl-api/storage"
)

type ACLAPIStorageSuite struct {
	suite.Suite
	SetupTestFunc func()
	Stor          storage.ACLAPIStorage
}

func (s *ACLAPIStorageSuite) SetupTest() {
	s.SetupTestFunc()
}

func (s *ACLAPIStorageSuite) TestAdd() {
	t := s.T()
	err := s.Stor.Add("r1", []storage.ACLIdPair{
		{ACLRuleID: "ar1", NetworkID: "n1"},
		{ACLRuleID: "ar2", NetworkID: "n2"},
		{ACLRuleID: "ar1", NetworkID: "n1"},
	})
	require.Nil(t, err)
	err = s.Stor.Add("r2", []storage.ACLIdPair{
		{ACLRuleID: "ar1", NetworkID: "n1"},
	})
	require.NoError(t, err)
	dbRule, err := s.Stor.Find("r1")
	require.Nil(t, err)
	expected := storage.ACLAPISyncedRule{
		RuleID: "r1",
		ACLIds: []storage.ACLIdPair{
			{ACLRuleID: "ar1", NetworkID: "n1"},
			{ACLRuleID: "ar2", NetworkID: "n2"},
		},
	}
	assert.Equal(t, expected, dbRule)
	err = s.Stor.Add("r1", []storage.ACLIdPair{
		{ACLRuleID: "ar1", NetworkID: "n1"},
		{ACLRuleID: "ar2", NetworkID: "n2"},
		{ACLRuleID: "ar1", NetworkID: "n1"},
	})
	require.Nil(t, err)
	dbRule, err = s.Stor.Find("r1")
	require.Nil(t, err)
	assert.Equal(t, expected, dbRule)
	dbRule, err = s.Stor.Find("r2")
	require.Nil(t, err)
	assert.Equal(t, storage.ACLAPISyncedRule{
		RuleID: "r2",
		ACLIds: []storage.ACLIdPair{
			{ACLRuleID: "ar1", NetworkID: "n1"},
		},
	}, dbRule)
}

func (s *ACLAPIStorageSuite) TestRemove() {
	t := s.T()
	err := s.Stor.Add("r1", []storage.ACLIdPair{
		{ACLRuleID: "ar1", NetworkID: "n1"},
		{ACLRuleID: "ar2", NetworkID: "n2"},
		{ACLRuleID: "ar1", NetworkID: "n1"},
	})
	require.Nil(t, err)
	err = s.Stor.Remove("r1", []storage.ACLIdPair{
		{ACLRuleID: "ar1", NetworkID: "n1"},
	})
	require.Nil(t, err)
	dbRule, err := s.Stor.Find("r1")
	require.Nil(t, err)
	expected := storage.ACLAPISyncedRule{
		RuleID: "r1",
		ACLIds: []storage.ACLIdPair{
			{ACLRuleID: "ar2", NetworkID: "n2"},
		},
	}
	assert.Equal(t, expected, dbRule)
}

func (s *ACLAPIStorageSuite) TestFind() {
	t := s.T()
	_, err := s.Stor.Find("r1")
	require.Equal(t, storage.ErrACLAPISyncedRuleNotFound, err)
}

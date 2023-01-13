// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storagetest

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tsuru/acl-api/api/types"
	"github.com/tsuru/acl-api/storage"
)

type RuleStorageSuite struct {
	suite.Suite
	SetupTestFunc func()
	Stor          storage.RuleStorage
}

func (s *RuleStorageSuite) SetupTest() {
	s.SetupTestFunc()
}

func (s *RuleStorageSuite) TestSave() {
	r := types.Rule{
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
	err := s.Stor.Save([]*types.Rule{&r}, false)
	require.Nil(s.T(), err)
	rules, err := s.Stor.FindAll(storage.FindOpts{})
	require.Nil(s.T(), err)
	require.Len(s.T(), rules, 1)
	assert.Equal(s.T(), []types.Rule{
		{
			RuleID: rules[0].RuleID,
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
		},
	}, rules)
}

func (s *RuleStorageSuite) TestFind() {
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
	err := s.Stor.Save([]*types.Rule{&r}, false)
	require.Nil(s.T(), err)
	rule, err := s.Stor.Find("1")
	require.Nil(s.T(), err)
	assert.Equal(s.T(), types.Rule{
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
		Metadata: map[string]string{},
		Created:  rule.Created,
	}, rule)
}

func (s *RuleStorageSuite) TestFindSourceTsuruApp() {
	r1 := types.Rule{
		RuleID: "1",
		Source: types.RuleType{
			TsuruApp: &types.TsuruAppRule{
				AppName: "myapp1",
			},
		},
		Destination: types.RuleType{
			ExternalDNS: &types.ExternalDNSRule{
				Name: "x.com",
			},
		},
	}
	r2 := types.Rule{
		RuleID: "2",
		Source: types.RuleType{
			TsuruApp: &types.TsuruAppRule{
				AppName: "myapp2",
			},
		},
		Destination: types.RuleType{
			ExternalDNS: &types.ExternalDNSRule{
				Name: "x.com",
			},
		},
	}
	err := s.Stor.Save([]*types.Rule{&r1, &r2}, false)
	require.Nil(s.T(), err)
	rule, err := s.Stor.FindAll(storage.FindOpts{
		SourceTsuruApp: "myapp1",
	})

	s.Require().NoError(err)
	s.Len(rule, 1)
	s.Equal("1", rule[0].RuleID)

	rule, err = s.Stor.FindAll(storage.FindOpts{
		SourceTsuruApp: "myapp2",
	})

	s.Require().NoError(err)
	s.Len(rule, 1)
	s.Equal("2", rule[0].RuleID)

}

func (s *RuleStorageSuite) TestDelete() {
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
	err := s.Stor.Save([]*types.Rule{&r}, false)
	require.Nil(s.T(), err)
	err = s.Stor.Delete(storage.DeleteOpts{ID: "1"})
	require.Nil(s.T(), err)
	rule, err := s.Stor.Find("1")
	require.Nil(s.T(), err)
	assert.Equal(s.T(), types.Rule{
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
		Created:  rule.Created,
	}, rule)
}

func (s *RuleStorageSuite) TestDeleteMetadata() {
	r := types.Rule{
		RuleID: "x",
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
			"a": "b",
		},
	}
	err := s.Stor.Save([]*types.Rule{&r}, false)
	require.Nil(s.T(), err)
	err = s.Stor.Delete(storage.DeleteOpts{Metadata: map[string]string{"a": "b"}})
	require.Nil(s.T(), err)
	rule, err := s.Stor.Find("x")
	require.Nil(s.T(), err)
	assert.Equal(s.T(), types.Rule{
		Removed:  true,
		RuleID:   "x",
		Metadata: map[string]string{"a": "b"},
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
		Created: rule.Created,
	}, rule)
}

func (s *RuleStorageSuite) TestDeleteMetadataMultiple() {
	r := types.Rule{
		RuleID: "x",
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
			"a": "b",
		},
	}
	err := s.Stor.Save([]*types.Rule{&r}, false)
	require.Nil(s.T(), err)
	r2 := r
	r2.RuleID = "y"
	err = s.Stor.Save([]*types.Rule{&r2}, false)
	require.Nil(s.T(), err)
	err = s.Stor.Delete(storage.DeleteOpts{Metadata: map[string]string{"a": "b"}})
	require.Nil(s.T(), err)
	rule, err := s.Stor.Find("x")
	require.Nil(s.T(), err)
	assert.Equal(s.T(), types.Rule{
		Removed:  true,
		RuleID:   "x",
		Metadata: map[string]string{"a": "b"},
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
		Created: rule.Created,
	}, rule)
	rule, err = s.Stor.Find("y")
	require.Nil(s.T(), err)
	assert.True(s.T(), rule.Removed)
}

func (s *RuleStorageSuite) TestDeleteNotFound() {
	err := s.Stor.Delete(storage.DeleteOpts{ID: "1"})
	require.Equal(s.T(), storage.ErrRuleNotFound, err)
}

func (s *RuleStorageSuite) TestFindNotFound() {
	_, err := s.Stor.Find("1")
	require.Equal(s.T(), storage.ErrRuleNotFound, err)
}

// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rule

import (
	"github.com/pkg/errors"
	"github.com/tsuru/acl-api/api/types"
	"github.com/tsuru/acl-api/external"
	aclKube "github.com/tsuru/acl-api/kubernetes"
	"github.com/tsuru/tsuru/provision/pool"
	"k8s.io/client-go/rest"
)

var (
	_ RuleLogic = &tsuruAppRuleLogic{}

	emptyRuleError = errors.New("rule must have an app name or a pool name")
)

type tsuruAppRuleLogic struct {
	rule        *types.TsuruAppRule
	tsuruClient external.TsuruClient
}

func (s *tsuruAppRuleLogic) isEmptyRule() bool {
	return s.rule.AppName == "" && s.rule.PoolName == ""
}

func (s *tsuruAppRuleLogic) getPool() (*pool.Pool, error) {
	poolName, err := s.getPoolName()
	if err != nil {
		return nil, err
	}
	return s.tsuruClient.PoolInfo(poolName)
}

func (s *tsuruAppRuleLogic) getPoolName() (string, error) {
	if s.isEmptyRule() {
		return "", emptyRuleError
	}
	if s.rule.AppName != "" {
		appInfo, err := s.tsuruClient.AppInfo(s.rule.AppName)
		if err != nil {
			return "", err
		}
		return appInfo.Pool, nil
	}
	return s.rule.PoolName, nil
}

func (s *tsuruAppRuleLogic) FriendlyName() string {
	if s.rule.AppName != "" {
		return s.rule.AppName
	}
	return s.rule.PoolName
}

func (s *tsuruAppRuleLogic) KubernetesRestConfig() (*rest.Config, error) {
	pool, err := s.getPool()
	if err != nil {
		return nil, err
	}
	if pool.Provisioner != "kubernetes" {
		return nil, nil
	}
	cluster, err := s.tsuruClient.PoolCluster(*pool)
	if err != nil {
		return nil, err
	}
	return aclKube.RestConfig(*cluster)
}

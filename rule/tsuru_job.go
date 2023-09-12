// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rule

import (
	"github.com/tsuru/acl-api/api/types"
	"github.com/tsuru/acl-api/external"
	aclKube "github.com/tsuru/acl-api/kubernetes"
	"github.com/tsuru/tsuru/provision/pool"
	"k8s.io/client-go/rest"
)

var (
	_ RuleLogic = &tsuruJobRuleLogic{}
)

type tsuruJobRuleLogic struct {
	rule        *types.TsuruJobRule
	tsuruClient external.TsuruClient
}

func (s *tsuruJobRuleLogic) isEmptyRule() bool {
	return s.rule.JobName == ""
}

func (s *tsuruJobRuleLogic) getPoolName() (string, error) {
	if s.isEmptyRule() {
		return "", emptyRuleError
	}
	jobInfo, err := s.tsuruClient.JobInfo(s.rule.JobName)
	if err != nil {
		return "", err
	}
	return jobInfo.Pool, nil

}

func (s *tsuruJobRuleLogic) getPool() (*pool.Pool, error) {
	poolName, err := s.getPoolName()
	if err != nil {
		return nil, err
	}
	return s.tsuruClient.PoolInfo(poolName)
}

func (s *tsuruJobRuleLogic) KubernetesRestConfig() (*rest.Config, string, error) {
	pool, err := s.getPool()
	if err != nil {
		return nil, "", err
	}
	if pool.Provisioner != "kubernetes" {
		return nil, "", nil
	}
	cluster, err := s.tsuruClient.PoolCluster(*pool)
	if err != nil {
		return nil, "", err
	}
	restConfig, err := aclKube.RestConfig(*cluster)
	if err != nil {
		return nil, "", err
	}
	return restConfig, pool.Name, nil
}

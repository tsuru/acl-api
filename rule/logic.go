// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rule

import (
	"sync"

	"github.com/tsuru/acl-api/api/types"
	"github.com/tsuru/acl-api/external"
	"k8s.io/client-go/rest"
)

type RuleLogic interface {
	KubernetesRestConfig() (*rest.Config, error)
}

type logicCache struct {
	sync.Mutex
	cache       map[string]RuleLogic
	tsuruClient external.TsuruClient
}

func (l *logicCache) logicFromRuleType(r types.RuleType) RuleLogic {
	if r.TsuruApp != nil {
		return &tsuruAppRuleLogic{rule: r.TsuruApp, tsuruClient: l.tsuruClient}
	}

	return nil
}

type LogicCache interface {
	LogicFromRule(r types.Rule) (src RuleLogic, err error)
}

func NewLogicCache() LogicCache {
	return &logicCache{
		tsuruClient: external.NewTsuruClient(),
	}
}

func (l *logicCache) LogicFromRule(r types.Rule) (src RuleLogic, err error) {
	l.Lock()
	defer l.Unlock()
	if l.cache == nil {
		l.cache = map[string]RuleLogic{}
	}

	srcKey, err := r.Source.CacheKey()
	if err != nil {
		return nil, err
	}
	srcLogic, ok := l.cache[srcKey]
	if !ok {
		srcLogic = l.logicFromRuleType(r.Source)
		l.cache[srcKey] = srcLogic
	}

	return srcLogic, nil
}

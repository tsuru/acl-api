// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rule

import (
	"reflect"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/tsuru/acl-api/api/types"
	"github.com/tsuru/acl-api/storage"
)

type RuleService interface {
	EngineRuleService
	Save(rules []*types.Rule, upsert bool) error
	FindMetadata(metadata map[string]string) ([]types.Rule, error)
	FindByRule(rule types.Rule) ([]types.Rule, error)
	FindByID(id string) (types.Rule, error)
	FindBySourceTsuruApp(appName string) ([]types.Rule, error)
	FindBySourceTsuruJob(jobName string) ([]types.Rule, error)
	Delete(id string) error
	DeleteMetadata(metadata map[string]string) error
	FindSyncs(ruleIDFilter []string) ([]types.RuleSyncInfo, error)
}

type EngineRuleService interface {
	FindAll() ([]types.Rule, error)
	SyncStart(after time.Duration, ruleID, engine string, force bool) (time.Duration, *types.RuleSyncInfo, error)
	SyncEnd(ruleSync types.RuleSyncInfo, syncData types.RuleSyncData) error
}

type ruleServiceImpl struct{}

func (s *ruleServiceImpl) Save(rules []*types.Rule, upsert bool) error {
	stor, err := storage.GetRuleStorage()
	if err != nil {
		return err
	}
	for _, r := range rules {
		err = validateRule(r)
		if err != nil {
			return err
		}
	}
	return stor.Save(rules, upsert)
}

func (s *ruleServiceImpl) FindAll() ([]types.Rule, error) {
	stor, err := storage.GetRuleStorage()
	if err != nil {
		return nil, err
	}
	return stor.FindAll(storage.FindOpts{})
}

func (s *ruleServiceImpl) FindMetadata(metadata map[string]string) ([]types.Rule, error) {
	stor, err := storage.GetRuleStorage()
	if err != nil {
		return nil, err
	}
	return stor.FindAll(storage.FindOpts{
		Metadata: metadata,
	})
}

func (s *ruleServiceImpl) FindBySourceTsuruApp(appName string) ([]types.Rule, error) {
	stor, err := storage.GetRuleStorage()
	if err != nil {
		return nil, err
	}
	return stor.FindAll(storage.FindOpts{
		SourceTsuruApp: appName,
	})
}

func (s *ruleServiceImpl) FindBySourceTsuruJob(jobName string) ([]types.Rule, error) {
	stor, err := storage.GetRuleStorage()
	if err != nil {
		return nil, err
	}
	return stor.FindAll(storage.FindOpts{
		SourceTsuruJob: jobName,
	})
}

func ruleTypeMatch(ruleType types.RuleType, filter types.RuleType) bool {
	if filter.ExternalDNS != nil {
		if ruleType.ExternalDNS == nil {
			return false
		}
		if filter.ExternalDNS.Name != "" && filter.ExternalDNS.Name != ruleType.ExternalDNS.Name {
			return false
		}
		if filter.ExternalDNS.Ports != nil && !reflect.DeepEqual(filter.ExternalDNS.Ports, ruleType.ExternalDNS.Ports) {
			return false
		}
	}
	if filter.ExternalIP != nil {
		if ruleType.ExternalIP == nil {
			return false
		}
		if filter.ExternalIP.IP != "" && filter.ExternalIP.IP != ruleType.ExternalIP.IP {
			return false
		}
		if filter.ExternalIP.Ports != nil && !reflect.DeepEqual(filter.ExternalIP.Ports, ruleType.ExternalIP.Ports) {
			return false
		}
	}
	if filter.TsuruApp != nil {
		if ruleType.TsuruApp == nil {
			return false
		}
		if filter.TsuruApp.AppName != "" && filter.TsuruApp.AppName != ruleType.TsuruApp.AppName {
			return false
		}
		if filter.TsuruApp.PoolName != "" && filter.TsuruApp.PoolName != ruleType.TsuruApp.PoolName {
			return false
		}
	}
	if filter.KubernetesService != nil {
		if ruleType.KubernetesService == nil {
			return false
		}
		if filter.KubernetesService.ServiceName != "" && filter.KubernetesService.ServiceName != ruleType.KubernetesService.ServiceName {
			return false
		}
		if filter.KubernetesService.Namespace != "" && filter.KubernetesService.Namespace != ruleType.KubernetesService.Namespace {
			return false
		}
	}
	return true
}

func ruleMatch(rule types.Rule, filter types.Rule) bool {
	if !ruleTypeMatch(rule.Source, filter.Source) {
		return false
	}
	if !ruleTypeMatch(rule.Destination, filter.Destination) {
		return false
	}
	return true
}

func (s *ruleServiceImpl) FindByRule(filter types.Rule) ([]types.Rule, error) {
	stor, err := storage.GetRuleStorage()
	if err != nil {
		return nil, err
	}
	allByMetadata, err := stor.FindAll(storage.FindOpts{
		Metadata: filter.Metadata,
		Creator:  filter.Creator,
	})
	if err != nil {
		return nil, err
	}
	var ret []types.Rule
	for _, r := range allByMetadata {
		if ruleMatch(r, filter) {
			ret = append(ret, r)
		}
	}
	return ret, nil
}

func (s *ruleServiceImpl) FindByID(id string) (types.Rule, error) {
	stor, err := storage.GetRuleStorage()
	if err != nil {
		return types.Rule{}, err
	}
	return stor.Find(id)
}

func (s *ruleServiceImpl) DeleteMetadata(metadata map[string]string) error {
	stor, err := storage.GetRuleStorage()
	if err != nil {
		return err
	}
	return stor.Delete(storage.DeleteOpts{Metadata: metadata})
}

func (s *ruleServiceImpl) Delete(id string) error {
	stor, err := storage.GetRuleStorage()
	if err != nil {
		return err
	}
	return stor.Delete(storage.DeleteOpts{ID: id})
}

func (s *ruleServiceImpl) FindSyncs(ruleIDFilter []string) ([]types.RuleSyncInfo, error) {
	stor, err := storage.GetSyncStorage()
	if err != nil {
		return nil, err
	}
	syncs, err := stor.Find(storage.SyncFindOpts{
		RuleIDs: ruleIDFilter,
	})
	if err != nil {
		return nil, err
	}
	return syncs, nil
}

var lockUpdaterInterval = 20 * time.Second

type lockUpdater struct {
	addSyncID    chan string
	removeSyncID chan string
	stopCh       chan struct{}
	syncIDSet    map[string]struct{}
}

func (l *lockUpdater) stop() {
	l.stopCh <- struct{}{}
}

func (l *lockUpdater) enqueue(id string) {
	l.addSyncID <- id
}

func (l *lockUpdater) dequeue(id string) {
	l.removeSyncID <- id
}

func (l *lockUpdater) run() {
	l.stopCh = make(chan struct{})
	l.addSyncID = make(chan string)
	l.removeSyncID = make(chan string)
	l.syncIDSet = make(map[string]struct{})
	logger := logrus.WithField("source", "lockUpdater")
	go func() {
		for {
			select {
			case id := <-l.addSyncID:
				l.syncIDSet[id] = struct{}{}
			case id := <-l.removeSyncID:
				delete(l.syncIDSet, id)
			case <-l.stopCh:
				return
			case <-time.After(lockUpdaterInterval):
			}
			if len(l.syncIDSet) == 0 {
				continue
			}
			syncs := make([]string, 0, len(l.syncIDSet))
			for k := range l.syncIDSet {
				syncs = append(syncs, k)
			}
			stor, err := storage.GetSyncStorage()
			if err != nil {
				logger.Errorf("unable to get sync storage: %v", err)
			}
			err = stor.PingSyncs(syncs)
			if err != nil {
				logger.Errorf("unable to update sync lock: %v", err)
			}
		}
	}()
}

var updater = &lockUpdater{}

func init() {
	updater.run()
}

func (s *ruleServiceImpl) SyncStart(after time.Duration, ruleID, engine string, force bool) (time.Duration, *types.RuleSyncInfo, error) {
	stor, err := storage.GetSyncStorage()
	if err != nil {
		return 0, nil, err
	}
	next, ruleSync, err := stor.StartSync(after, ruleID, engine, force)
	if err != nil {
		return next, nil, err
	}
	updater.enqueue(ruleSync.SyncID)
	return next, ruleSync, nil
}

func (s *ruleServiceImpl) SyncEnd(ruleSync types.RuleSyncInfo, syncData types.RuleSyncData) error {
	updater.dequeue(ruleSync.SyncID)
	stor, err := storage.GetSyncStorage()
	if err != nil {
		return err
	}
	return stor.EndSync(ruleSync, syncData)
}

var GetServiceForEngine = func() EngineRuleService {
	return GetService()
}

var GetService = func() RuleService {
	return &ruleServiceImpl{}
}

func validateRule(r *types.Rule) error {
	err := r.Source.Validate()
	if err != nil {
		return errors.Wrap(err, "source")
	}
	err = r.Destination.Validate()
	if err != nil {
		return errors.Wrap(err, "destination")
	}
	return nil
}

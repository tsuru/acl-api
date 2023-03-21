// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package service

import (
	"errors"
	"fmt"

	"github.com/tsuru/acl-api/api/types"
	"github.com/tsuru/acl-api/rule"
	"github.com/tsuru/acl-api/storage"
)

const (
	OwnerAclFromHell = "aclfromhell"
)

var ErrRuleAlreadyExists = errors.New("rule already exists")

type Service interface {
	Create(instance types.ServiceInstance) error
	List() ([]types.ServiceInstance, error)
	Find(instanceName string) (types.ServiceInstance, error)
	Delete(instanceName string) error
	AddRule(instanceName string, r *types.ServiceRule) ([]types.Rule, error)
	RemoveRule(instanceName string, ruleID string) error
	AddApp(instanceName string, appName string) ([]types.Rule, error)
	RemoveApp(instanceName string, appName string) error
	AddJob(instanceName string, appName string) ([]types.Rule, error)
	RemoveJob(instanceName string, appName string) error
}

type serviceImpl struct{}

func (s *serviceImpl) Create(instance types.ServiceInstance) error {
	stor, err := storage.GetServiceStorage()
	if err != nil {
		return err
	}
	return stor.Create(instance)
}

func (s *serviceImpl) List() ([]types.ServiceInstance, error) {
	stor, err := storage.GetServiceStorage()
	if err != nil {
		return nil, err
	}
	return stor.List()
}

func (s *serviceImpl) Find(instanceName string) (types.ServiceInstance, error) {
	stor, err := storage.GetServiceStorage()
	if err != nil {
		return types.ServiceInstance{}, err
	}
	return stor.Find(instanceName)
}

func (s *serviceImpl) Delete(instanceName string) error {
	stor, err := storage.GetServiceStorage()
	if err != nil {
		return err
	}
	ruleSvc := rule.GetService()
	err = ruleSvc.DeleteMetadata(map[string]string{
		"owner":         OwnerAclFromHell,
		"instance-name": instanceName,
	})
	if err != nil && err != storage.ErrRuleNotFound {
		return err
	}
	return stor.Delete(instanceName)
}

func (s *serviceImpl) AddRule(instanceName string, r *types.ServiceRule) ([]types.Rule, error) {
	err := r.Destination.Validate()
	if err != nil {
		return nil, err
	}
	stor, err := storage.GetServiceStorage()
	if err != nil {
		return nil, err
	}

	service, err := stor.Find(instanceName)
	if err != nil {
		return nil, err
	}

	for _, baseRule := range service.BaseRules {
		if baseRule.Removed {
			continue
		}

		if baseRule.Equals(r) {
			return nil, ErrRuleAlreadyExists
		}
	}

	err = stor.AddRule(instanceName, r)
	if err != nil {
		return nil, err
	}
	return syncRules(instanceName)
}

func ruleMetadata(baseID, instanceName string) map[string]string {
	return map[string]string{
		"owner":         OwnerAclFromHell,
		"base-ruleid":   baseID,
		"instance-name": instanceName,
	}
}
func ruleAppMetadata(baseID, instanceName, appName string) map[string]string {
	r := ruleMetadata(baseID, instanceName)
	r["app-name"] = appName
	return r
}

func ruleJobMetadata(baseID, instanceName, jobName string) map[string]string {
	r := ruleMetadata(baseID, instanceName)
	r["job-name"] = jobName
	return r
}

func (s *serviceImpl) RemoveRule(instanceName string, ruleID string) error {
	stor, err := storage.GetServiceStorage()
	if err != nil {
		return err
	}
	ruleSvc := rule.GetService()
	err = ruleSvc.DeleteMetadata(map[string]string{
		"owner":         OwnerAclFromHell,
		"instance-name": instanceName,
		"base-ruleid":   ruleID,
	})
	if err != nil && err != storage.ErrRuleNotFound {
		return err
	}
	return stor.RemoveRule(instanceName, ruleID)
}

func (s *serviceImpl) AddApp(instanceName string, appName string) ([]types.Rule, error) {
	stor, err := storage.GetServiceStorage()
	if err != nil {
		return nil, err
	}
	err = stor.AddApp(instanceName, appName)
	if err != nil {
		return nil, err
	}
	return syncRules(instanceName)
}

func (s *serviceImpl) AddJob(instanceName string, jobName string) ([]types.Rule, error) {
	stor, err := storage.GetServiceStorage()
	if err != nil {
		return nil, err
	}
	err = stor.AddJob(instanceName, jobName)
	if err != nil {
		return nil, err
	}
	return syncRules(instanceName)
}

func (s *serviceImpl) RemoveApp(instanceName string, appName string) error {
	stor, err := storage.GetServiceStorage()
	if err != nil {
		return err
	}
	ruleSvc := rule.GetService()
	err = ruleSvc.DeleteMetadata(map[string]string{
		"owner":         OwnerAclFromHell,
		"instance-name": instanceName,
		"app-name":      appName,
	})
	if err != nil && err != storage.ErrRuleNotFound {
		return err
	}
	return stor.RemoveApp(instanceName, appName)
}

func (s *serviceImpl) RemoveJob(instanceName string, jobName string) error {
	stor, err := storage.GetServiceStorage()
	if err != nil {
		return err
	}
	ruleSvc := rule.GetService()
	err = ruleSvc.DeleteMetadata(map[string]string{
		"owner":         OwnerAclFromHell,
		"instance-name": instanceName,
		"job-name":      jobName,
	})
	if err != nil && err != storage.ErrRuleNotFound {
		return err
	}
	return stor.RemoveJob(instanceName, jobName)
}

var GetService = func() Service {
	return &serviceImpl{}
}

func expandRules(instanceName string) ([]*types.Rule, error) {
	stor, err := storage.GetServiceStorage()
	if err != nil {
		return nil, err
	}
	instance, err := stor.Find(instanceName)
	if err != nil {
		return nil, err
	}
	var allRules []*types.Rule
	for _, r := range instance.BaseRules {
		baseID := r.RuleID
		for _, appName := range instance.BindApps {
			appRule := r
			appRule.Source = types.RuleType{
				TsuruApp: &types.TsuruAppRule{
					AppName: appName,
				},
			}
			appRule.RuleID = fmt.Sprintf("%s-%s", baseID, appName)
			appRule.Metadata = ruleAppMetadata(baseID, instanceName, appName)
			appRule.Creator = r.Creator
			allRules = append(allRules, &appRule.Rule)
		}

		for _, jobName := range instance.BindJobs {
			appRule := r
			appRule.Source = types.RuleType{
				TsuruJob: &types.TsuruJobRule{
					JobName: jobName,
				},
			}
			appRule.RuleID = fmt.Sprintf("job-%s-%s", baseID, jobName)
			appRule.Metadata = ruleJobMetadata(baseID, instanceName, jobName)
			appRule.Creator = r.Creator
			allRules = append(allRules, &appRule.Rule)
		}
	}
	return allRules, nil
}

func syncRules(instanceName string) ([]types.Rule, error) {
	rules, err := expandRules(instanceName)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, nil
	}
	ruleSvc := rule.GetService()
	err = ruleSvc.Save(rules, true)
	if err != nil {
		return nil, err
	}
	insertedRules := make([]types.Rule, len(rules))
	for i, r := range rules {
		insertedRules[i] = *r
	}
	return insertedRules, nil
}

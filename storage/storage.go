// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage

import (
	"net"
	"time"

	"github.com/pkg/errors"
	"github.com/tsuru/acl-api/api/types"
)

var (
	ErrRuleNotFound = errors.New("rule not found")

	ErrInstanceNotFound      = errors.New("instance not found")
	ErrInstanceAlreadyExists = errors.New("instance already exists")

	ErrSyncStorageLocked = errors.New("sync already locked")

	ErrACLAPISyncedRuleNotFound = errors.New("aclapi synced rule not found")
)

type ServiceStorage interface {
	Create(instance types.ServiceInstance) error
	List() ([]types.ServiceInstance, error)
	Find(instanceName string) (types.ServiceInstance, error)
	Delete(instanceName string) error
	AddRule(instanceName string, r *types.ServiceRule) error
	RemoveRule(instanceName string, ruleID string) error
	AddApp(instanceName string, appName string) error
	RemoveApp(instanceName string, appName string) error
	AddJob(instanceName string, jobName string) error
	RemoveJob(instanceName string, jobName string) error
}

type DeleteOpts struct {
	ID       string
	Metadata map[string]string
}

type FindOpts struct {
	Metadata map[string]string
	Creator  string

	SourceTsuruApp string
	SourceTsuruJob string
}

type SyncFindOpts struct {
	RuleIDs []string
	Engines []string
	Limit   int
}

type SyncStorage interface {
	Find(opts SyncFindOpts) ([]types.RuleSyncInfo, error)
	StartSync(after time.Duration, ruleID, engine string, force bool) (time.Duration, *types.RuleSyncInfo, error)
	PingSyncs(ruleSyncIDs []string) error
	EndSync(ruleSync types.RuleSyncInfo, syncData types.RuleSyncData) error
	SetLockExpireTime(timeout time.Duration) time.Duration
}

type RuleStorage interface {
	Find(id string) (types.Rule, error)
	Save(rules []*types.Rule, upsert bool) error
	FindAll(opts FindOpts) ([]types.Rule, error)
	Delete(opts DeleteOpts) error
}

type ACLAPISyncedRule struct {
	RuleID string
	ACLIds []ACLIdPair
}

type ACLIdPair struct {
	NetworkID string
	ACLRuleID string
}

type ACLAPIStorage interface {
	Find(ruleID string) (ACLAPISyncedRule, error)
	Add(ruleID string, aclIDs []ACLIdPair) error
	Remove(ruleID string, aclIDs []ACLIdPair) error
}

type StoredIP struct {
	IP         net.IP
	ValidUntil time.Time
}

var GetSyncStorage = func() (SyncStorage, error) {
	return nil, errors.New("no sync storage imported")
}

var GetRuleStorage = func() (RuleStorage, error) {
	return nil, errors.New("no rule storage imported")
}

var GetServiceStorage = func() (ServiceStorage, error) {
	return nil, errors.New("no service storage imported")
}

var GetACLAPIStorage = func() (ACLAPIStorage, error) {
	return nil, errors.New("no acl api storage imported")
}

// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package engine

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/tsuru/acl-api/api/types"
	"github.com/tsuru/acl-api/rule"
	"github.com/tsuru/acl-api/storage"
)

const (
	promNamespace = "acl_api"
	promSubsystem = "engine"
)

var (
	ruleSyncDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: promNamespace,
		Subsystem: promSubsystem,
		Name:      "rule_sync_duration_seconds",
		Help:      "Rule sync duration",
		Buckets:   prometheus.ExponentialBuckets(0.1, 2.4, 10),
	}, []string{"engine"})

	fullSyncDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: promNamespace,
		Subsystem: promSubsystem,
		Name:      "full_sync_duration_seconds",
		Help:      "Full engine sync duration",
		Buckets:   prometheus.ExponentialBuckets(2, 2.9, 10),
	}, []string{"engine"})

	ruleSyncFailuresTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: promNamespace,
		Subsystem: promSubsystem,
		Name:      "rule_sync_failures_total",
		Help:      "The number of rule sync failures",
	}, []string{"engine"})
)

type Engine interface {
	Sync(r types.Rule) (interface{}, error)
	Name() string
}

type EngineWithFilter interface {
	Allowed(r types.Rule) (bool, error)
}

type EngineWithHooks interface {
	BeforeSync(logicCache rule.LogicCache) error
	AfterSync() error
}

var (
	enabledEngines []func() Engine
	quitCh         = make(chan struct{})
)

func syncRule(log *logrus.Entry, ruleSvc rule.EngineRuleService, e Engine, r types.Rule, force bool) (err error) {
	if filterEngine, ok := e.(EngineWithFilter); ok {
		var allowed bool
		allowed, err = filterEngine.Allowed(r)
		if err != nil {
			return err
		}
		if !allowed {
			log.Debug("Rule not allowed in current worker")
			return nil
		}
	}
	syncInterval := viper.GetDuration("sync.interval")
	_, ruleSync, err := ruleSvc.SyncStart(syncInterval, r.RuleID, e.Name(), force)
	if err != nil {
		if err == storage.ErrSyncStorageLocked {
			return nil
		}
		return err
	}
	log = log.WithField("syncid", ruleSync.SyncID)
	var syncData types.RuleSyncData
	defer func() {
		syncData.EndTime = time.Now().UTC()
		syncData.Successful = err == nil
		syncData.Removed = r.Removed
		if err != nil {
			syncData.Error = err.Error()
		}
		syncEndErr := ruleSvc.SyncEnd(*ruleSync, syncData)
		if syncEndErr != nil {
			log.Errorf("unable to mark sync end for rule %v: %v", r.String(), syncEndErr)
		}
	}()
	syncData.StartTime = time.Now().UTC()
	latestSync := ruleSync.LatestSync()
	if latestSync != nil {
		if r.Removed && latestSync.Removed && latestSync.Successful {
			// Nothing to do, removal already synced
			return nil
		}
	}
	obj, err := e.Sync(r)
	if data, jsonErr := json.Marshal(obj); obj != nil && jsonErr == nil {
		syncData.SyncResult = string(data)
	}
	if err != nil {
		return err
	}
	return nil
}

func engineSync(e Engine, rules []types.Rule, logicCache rule.LogicCache, force bool) {
	log := logrus.WithField("engine", e.Name())
	fullTimer := prometheus.NewTimer(fullSyncDuration.WithLabelValues(e.Name()))
	defer fullTimer.ObserveDuration()
	hooksEngine, _ := e.(EngineWithHooks)
	if hooksEngine != nil {
		err := hooksEngine.BeforeSync(logicCache)
		if err != nil {
			log.Errorf("unable to run before sync in engine %v: %v", e.Name(), err)
		}
	}
	ruleSvc := rule.GetServiceForEngine()
	for _, r := range rules {
		ruleLog := log.WithField("ruleid", r.RuleID)
		ruleLog.Info("Starting single rule sync")
		ruleTimer := prometheus.NewTimer(ruleSyncDuration.WithLabelValues(e.Name()))
		err := syncRule(ruleLog, ruleSvc, e, r, force)
		ruleTimer.ObserveDuration()
		if err != nil {
			ruleSyncFailuresTotal.WithLabelValues(e.Name()).Inc()
			ruleLog.Errorf("error syncing rule %v: %v", r.String(), err)
		}
	}
	if hooksEngine != nil {
		err := hooksEngine.AfterSync()
		if err != nil {
			log.Errorf("unable to run after sync in engine %v: %v", e.Name(), err)
		}
	}
}

func syncAllRules() error {
	logrus.Info("Starting sync engines")
	defer logrus.Info("Done sync engines")
	ruleSvc := rule.GetServiceForEngine()
	rules, err := ruleSvc.FindAll()
	if err != nil {
		return err
	}
	SyncRules(rules, false)
	return nil
}

func EnableEngine(eng func() Engine) {
	enabledEngines = append(enabledEngines, eng)
}

func ShutdownPeriodicSync(ctx context.Context) error {
	select {
	case quitCh <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func RunPeriodicSync() {
	logrus.Info("Starting sync loop")
	if viper.GetBool("sync.disabled") {
		return
	}

	for {
		syncInterval := viper.GetDuration("sync.interval")
		err := syncAllRules()
		if err != nil {
			logrus.Errorf("error trying to run sync engines: %v", err)
		}
		select {
		case <-time.After(syncInterval):
		case <-quitCh:
			logrus.Info("Stopping sync loop")
			return
		}
	}
}

func SyncRules(rules []types.Rule, force bool) {
	logicCache := rule.NewLogicCache()
	wg := sync.WaitGroup{}
	for _, eFactory := range enabledEngines {
		e := eFactory()
		wg.Add(1)
		go func(e Engine) {
			defer wg.Done()
			engineSync(e, rules, logicCache, force)
		}(e)
	}
	wg.Wait()
}

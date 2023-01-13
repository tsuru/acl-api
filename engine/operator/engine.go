// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package operator

import (
	"context"
	"encoding/json"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/sirupsen/logrus"
	"github.com/tsuru/acl-api/api/types"
	"github.com/tsuru/acl-api/engine"
	"github.com/tsuru/acl-api/kubernetes"
	aclKube "github.com/tsuru/acl-api/kubernetes"
	"github.com/tsuru/acl-api/rule"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sTypes "k8s.io/apimachinery/pkg/types"
)

var (
	_ engine.Engine          = &ACLOperatorEngine{}
	_ engine.EngineWithHooks = &ACLOperatorEngine{}

	engineName = "acl-operator"

	logger = logrus.WithField("engine", engineName)
)

const (
	lastUpdatedAnnotation = "acl-api.tsuru.io/last-updated"
)

type ACLOperatorEngine struct {
	logicCache rule.LogicCache
}

func (e *ACLOperatorEngine) Name() string {
	return engineName
}

func (e *ACLOperatorEngine) BeforeSync(logicCache rule.LogicCache) error {
	e.logicCache = logicCache
	return nil
}

func (e *ACLOperatorEngine) AfterSync() error {
	return nil
}

func (e *ACLOperatorEngine) Sync(r types.Rule) (interface{}, error) {
	ctx := context.TODO()
	log := logger.WithField("ruleid", r.RuleID)

	if r.Source.TsuruApp == nil {
		return nil, nil
	}

	source, err := e.logicCache.LogicFromRule(r)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, nil
	}

	restConfig, err := source.KubernetesRestConfig()
	if err != nil {
		return nil, err
	}

	if restConfig == nil {
		log.Debugf("Ignoring rule, not a kubernetes source")
		return nil, nil
	}

	tsuruClient, err := kubernetes.GetTsuruClientWithRestConfig(restConfig)
	if err != nil {
		return "", err
	}

	tsuruApp := r.Source.TsuruApp.AppName
	namespace := aclKube.DefaultNamespace()

	appCR, err := tsuruClient.TsuruV1().Apps(namespace).Get(ctx, tsuruApp, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}

	originalData, err := json.Marshal(appCR)
	if err != nil {
		return "", err
	}

	lastUpdatedStr := appCR.Annotations[lastUpdatedAnnotation]
	var lastUpdated time.Time
	needsUpdate := false

	if lastUpdatedStr == "" {
		needsUpdate = true
	} else {
		lastUpdated, err = time.Parse(time.RFC3339, lastUpdatedStr)
		if err != nil {
			return "", err
		}

		if time.Now().UTC().After(lastUpdated.Add(time.Minute)) {
			needsUpdate = true
		}
	}

	if needsUpdate {
		if appCR.Annotations == nil {
			appCR.Annotations = map[string]string{}
		}
		appCR.Annotations[lastUpdatedAnnotation] = time.Now().UTC().Format(time.RFC3339)

		updatedData, err := json.Marshal(appCR)
		if err != nil {
			return "", err
		}

		data, err := jsonpatch.CreateMergePatch(originalData, updatedData)
		if err != nil {
			return "", err
		}

		_, err = tsuruClient.TsuruV1().Apps(namespace).Patch(ctx, tsuruApp, k8sTypes.MergePatchType, data, metav1.PatchOptions{})
		if err != nil {
			return "", err
		}

		return "triggered acl-operator", nil
	}

	return "triggered acl-operator in the last minute", nil
}

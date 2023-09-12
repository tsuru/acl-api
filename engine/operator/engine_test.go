// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package operator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/acl-api/api/types"
	aclKube "github.com/tsuru/acl-api/kubernetes"
	"github.com/tsuru/acl-api/rule"
	v1 "github.com/tsuru/tsuru/provision/kubernetes/pkg/apis/tsuru/v1"
	tsuruv1clientset "github.com/tsuru/tsuru/provision/kubernetes/pkg/client/clientset/versioned"
	faketsuru "github.com/tsuru/tsuru/provision/kubernetes/pkg/client/clientset/versioned/fake"
	"github.com/tsuru/tsuru/types/provision"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	fakeK8s "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func mockTsuruClient() (cli tsuruv1clientset.Interface, undo func()) {
	oldTsuruClient := aclKube.GetTsuruClientWithRestConfig
	oldRestConfig := aclKube.RestConfig
	tsuruClient := faketsuru.NewSimpleClientset()

	aclKube.GetTsuruClientWithRestConfig = func(config *rest.Config) (tsuruv1clientset.Interface, error) {
		return tsuruClient, nil
	}

	aclKube.RestConfig = func(cluster provision.Cluster) (*rest.Config, error) {
		return &rest.Config{}, nil
	}

	return tsuruClient, func() {
		aclKube.GetTsuruClientWithRestConfig = oldTsuruClient
		aclKube.RestConfig = oldRestConfig
	}
}

func mockK8sClient() (clientset *fakeK8s.Clientset, undo func()) {

	oldClientWithRestConfig := aclKube.GetClientWithRestConfig
	k8sClient := fakeK8s.NewSimpleClientset()
	oldRestConfig := aclKube.RestConfig

	aclKube.GetClientWithRestConfig = func(config *rest.Config) (kubernetes.Interface, error) {
		return k8sClient, nil
	}

	aclKube.RestConfig = func(cluster provision.Cluster) (*rest.Config, error) {
		return &rest.Config{}, nil
	}

	return k8sClient, func() {
		aclKube.GetClientWithRestConfig = oldClientWithRestConfig
		aclKube.RestConfig = oldRestConfig
	}
}

func mockTsuruAPI() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apps/app1":
			w.Write([]byte(`{"name": "app1", "pool": "p1"}`))
		case "/pools/p1":
			w.Write([]byte(`{"name": "p1", "provisioner": "kubernetes"}`))
		case "/provisioner/clusters":
			w.Write([]byte(`[{"name": "c1", "default": true, "provisioner": "kubernetes"}]`))
		case "/jobs/job1":
			w.Write([]byte(`{"job": {"name": "job1", "pool": "p1"}}`))

		default:
			panic("URL " + r.URL.Path + " is not mocked")
		}
	}))
}

func TestACLOperatorEngine_SyncApp(t *testing.T) {
	ctx := context.TODO()
	tsuruCli, undo := mockTsuruClient()
	defer undo()

	tsuruCli.TsuruV1().Apps("default").Create(ctx, &v1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "app1",
		},
		Spec: v1.AppSpec{
			NamespaceName: "default",
		},
	}, metav1.CreateOptions{})

	srv := mockTsuruAPI()
	defer srv.Close()

	viper.Set("tsuru.host", srv.URL)
	viper.Set("kubernetes.namespace", "default")

	e := &ACLOperatorEngine{
		logicCache: rule.NewLogicCache(),
	}
	result, err := e.Sync(types.Rule{
		RuleID: "1",
		Source: types.RuleType{
			TsuruApp: &types.TsuruAppRule{
				AppName: "app1",
			},
		},
		Destination: types.RuleType{
			TsuruApp: &types.TsuruAppRule{
				AppName: "app2",
			},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "triggered acl-operator", result)

	app, err := tsuruCli.TsuruV1().Apps("default").Get(ctx, "app1", metav1.GetOptions{})
	require.NoError(t, err)

	assert.NotEqual(t, "", app.Annotations["acl-api.tsuru.io/last-updated"])
}

func TestACLOperatorEngine_SyncAvoidFrequentUpdates(t *testing.T) {
	ctx := context.TODO()
	tsuruCli, undo := mockTsuruClient()
	defer undo()

	lastUpdated := time.Now().UTC().Add(time.Second * -30).Format(time.RFC3339)

	tsuruCli.TsuruV1().Apps("default").Create(ctx, &v1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "app1",
			Annotations: map[string]string{
				"acl-api.tsuru.io/last-updated": lastUpdated,
			},
		},
		Spec: v1.AppSpec{
			NamespaceName: "default",
		},
	}, metav1.CreateOptions{})

	srv := mockTsuruAPI()
	defer srv.Close()

	viper.Set("tsuru.host", srv.URL)
	viper.Set("kubernetes.namespace", "default")

	e := &ACLOperatorEngine{
		logicCache: rule.NewLogicCache(),
	}
	result, err := e.Sync(types.Rule{
		RuleID: "1",
		Source: types.RuleType{
			TsuruApp: &types.TsuruAppRule{
				AppName: "app1",
			},
		},
		Destination: types.RuleType{
			TsuruApp: &types.TsuruAppRule{
				AppName: "app2",
			},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "triggered acl-operator in the last minute", result)

	app, err := tsuruCli.TsuruV1().Apps("default").Get(ctx, "app1", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, lastUpdated, app.Annotations["acl-api.tsuru.io/last-updated"])
}

func TestACLOperatorEngine_SyncStaleUpdate(t *testing.T) {
	ctx := context.TODO()
	tsuruCli, undo := mockTsuruClient()
	defer undo()

	lastUpdated := time.Now().UTC().Add(time.Minute * -30).Format(time.RFC3339)

	tsuruCli.TsuruV1().Apps("default").Create(ctx, &v1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "app1",
			Annotations: map[string]string{
				"acl-api.tsuru.io/last-updated": lastUpdated,
			},
		},
		Spec: v1.AppSpec{
			NamespaceName: "default",
		},
	}, metav1.CreateOptions{})

	srv := mockTsuruAPI()
	defer srv.Close()

	viper.Set("tsuru.host", srv.URL)
	viper.Set("kubernetes.namespace", "default")

	e := &ACLOperatorEngine{
		logicCache: rule.NewLogicCache(),
	}
	result, err := e.Sync(types.Rule{
		RuleID: "1",
		Source: types.RuleType{
			TsuruApp: &types.TsuruAppRule{
				AppName: "app1",
			},
		},
		Destination: types.RuleType{
			TsuruApp: &types.TsuruAppRule{
				AppName: "app2",
			},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "triggered acl-operator", result)

	app, err := tsuruCli.TsuruV1().Apps("default").Get(ctx, "app1", metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotEqual(t, lastUpdated, app.Annotations["acl-api.tsuru.io/last-updated"])
}

func TestACLOperatorEngine_SyncRecentCreated(t *testing.T) {
	ctx := context.TODO()
	tsuruCli, undo := mockTsuruClient()
	defer undo()

	lastUpdated := time.Now().UTC().Add(time.Second * -30).Format(time.RFC3339)

	tsuruCli.TsuruV1().Apps("default").Create(ctx, &v1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "app1",
			Annotations: map[string]string{
				"acl-api.tsuru.io/last-updated": lastUpdated,
			},
		},
		Spec: v1.AppSpec{
			NamespaceName: "default",
		},
	}, metav1.CreateOptions{})

	srv := mockTsuruAPI()
	defer srv.Close()

	viper.Set("tsuru.host", srv.URL)
	viper.Set("kubernetes.namespace", "default")

	e := &ACLOperatorEngine{
		logicCache: rule.NewLogicCache(),
	}
	result, err := e.Sync(types.Rule{
		RuleID: "1",
		Source: types.RuleType{
			TsuruApp: &types.TsuruAppRule{
				AppName: "app1",
			},
		},
		Destination: types.RuleType{
			TsuruApp: &types.TsuruAppRule{
				AppName: "app2",
			},
		},
		Created: time.Now().UTC(),
	})

	require.NoError(t, err)
	assert.Equal(t, "triggered acl-operator", result)

	app, err := tsuruCli.TsuruV1().Apps("default").Get(ctx, "app1", metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotEqual(t, lastUpdated, app.Annotations["acl-api.tsuru.io/last-updated"])
}

// Jobs

func TestACLOperatorEngine_SyncJob(t *testing.T) {
	ctx := context.TODO()
	k8sCli, undo := mockK8sClient()
	defer undo()

	cronJobNamespace := k8sCli.BatchV1().CronJobs("tsuru-p1")
	cronJobNamespace.Create(ctx, &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: "job1",
		},
		Spec: batchv1.CronJobSpec{},
	}, metav1.CreateOptions{})

	srv := mockTsuruAPI()
	defer srv.Close()

	viper.Set("tsuru.host", srv.URL)

	e := &ACLOperatorEngine{
		logicCache: rule.NewLogicCache(),
	}
	result, err := e.Sync(types.Rule{
		RuleID: "1",
		Source: types.RuleType{
			TsuruJob: &types.TsuruJobRule{
				JobName: "job1",
			},
		},
		Destination: types.RuleType{
			TsuruApp: &types.TsuruAppRule{
				AppName: "app2",
			},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "triggered acl-operator", result)

	app, err := cronJobNamespace.Get(ctx, "job1", metav1.GetOptions{})
	require.NoError(t, err)

	assert.NotEqual(t, "", app.Annotations["acl-api.tsuru.io/last-updated"])
}

// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package external

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/tsuru/tsuru/app"
	"github.com/tsuru/tsuru/provision/pool"
	jobTypes "github.com/tsuru/tsuru/types/job"
	provTypes "github.com/tsuru/tsuru/types/provision"
)

const (
	kubernetesProvisioner = "kubernetes"
)

var (
	ErrNotKubernetesPool = errors.New("not a kubernetes pool")
	ErrClusterNotFound   = errors.New("cluster not found")
)

type TsuruClient interface {
	PoolCluster(tsuruPool pool.Pool) (*provTypes.Cluster, error)
	Cluster(clusterName string) (*provTypes.Cluster, error)
	JobInfo(jobName string) (*jobTypes.Job, error)
	AppInfo(appName string) (*app.App, error)
	PoolInfo(poolName string) (*pool.Pool, error)
	Clusters() ([]provTypes.Cluster, error)
}

func NewTsuruClient() TsuruClient {
	return &tsuruClient{
		BaseHTTPClient: &BaseHTTPClient{
			URL:    viper.GetString("tsuru.host"),
			Token:  viper.GetString("tsuru.token"),
			Logger: logrus.WithField("http-client", "tsuru"),
		},
		appInfoCache: map[string]*cachedApp{},
		poolCache:    map[string]*cachedPool{},
	}
}

type tsuruClient struct {
	sync.Mutex
	*BaseHTTPClient
	clustersCache []provTypes.Cluster
	appInfoCache  map[string]*cachedApp
	jobInfoCache  map[string]*cachedJob
	poolCache     map[string]*cachedPool
}

func (t *tsuruClient) PoolCluster(tsuruPool pool.Pool) (*provTypes.Cluster, error) {
	if tsuruPool.Provisioner != kubernetesProvisioner {
		return nil, ErrNotKubernetesPool
	}
	clusters, err := t.Clusters()
	if err != nil {
		return nil, err
	}
	var chosenCluster *provTypes.Cluster
	for i, c := range clusters {
		if c.Provisioner != kubernetesProvisioner {
			continue
		}
		if c.Default {
			chosenCluster = &clusters[i]
		}
		for _, p := range c.Pools {
			if p == tsuruPool.Name {
				return &c, nil
			}
		}
	}
	if chosenCluster == nil {
		return nil, ErrClusterNotFound
	}
	return chosenCluster, nil
}

func (t *tsuruClient) Cluster(clusterName string) (*provTypes.Cluster, error) {
	clusters, err := t.Clusters()
	if err != nil {
		return nil, err
	}
	for _, c := range clusters {
		if c.Provisioner != kubernetesProvisioner {
			continue
		}
		if c.Name == clusterName {
			return &c, nil
		}
	}
	return nil, ErrClusterNotFound
}

func (t *tsuruClient) AppInfo(appName string) (*app.App, error) {
	t.Lock()
	data, ok := t.appInfoCache[appName]
	if !ok {
		data = &cachedApp{cachedBase: cachedBase{cli: t}}
		t.appInfoCache[appName] = data
	}
	t.Unlock()
	return data.appInfo(appName)
}

func (t *tsuruClient) JobInfo(jobName string) (*jobTypes.Job, error) {
	t.Lock()
	data, ok := t.jobInfoCache[jobName]
	if !ok {
		data = &cachedJob{cachedBase: cachedBase{cli: t}}
		t.jobInfoCache[jobName] = data
	}
	t.Unlock()
	return data.jobInfo(jobName)
}

func (t *tsuruClient) PoolInfo(poolName string) (*pool.Pool, error) {
	t.Lock()
	data, ok := t.poolCache[poolName]
	if !ok {
		data = &cachedPool{cachedBase: cachedBase{cli: t}}
		t.poolCache[poolName] = data
	}
	t.Unlock()
	return data.poolInfo(poolName)
}

func (t *tsuruClient) Clusters() ([]provTypes.Cluster, error) {
	t.Lock()
	defer t.Unlock()
	if t.clustersCache != nil {
		return t.clustersCache, nil
	}
	var clusters []provTypes.Cluster
	err := t.doRequest(http.MethodGet, "/provisioner/clusters", &clusters)
	if err != nil {
		return nil, err
	}
	t.clustersCache = clusters
	return t.clustersCache, nil
}

func (t *tsuruClient) doRequest(method, url string, response interface{}) error {
	data, err := t.DoRequestData(method, url, nil, nil)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, response)
	if err != nil {
		return errors.Wrapf(err, "unable to unmarshal data %q", data)
	}
	return nil
}

type cachedBase struct {
	sync.Mutex
	cli *tsuruClient
}

type cachedApp struct {
	cachedBase
	result      *app.App
	cachedError error
}

func (c *cachedApp) appInfo(appName string) (*app.App, error) {
	c.Lock()
	defer c.Unlock()
	if c.result != nil {
		return c.result, nil
	}
	if c.cachedError != nil {
		return nil, c.cachedError
	}
	var appData app.App
	err := c.cli.doRequest(http.MethodGet, "/apps/"+appName, &appData)
	if err != nil {
		if httpErr, ok := errors.Cause(err).(*HTTPError); ok {
			if httpErr.StatusCode == http.StatusNotFound {
				c.cachedError = err
			}
		}
		return nil, err
	}
	if appData.Pool == "" || appData.Name == "" {
		return nil, errors.Errorf("empty data for app %q", appName)
	}
	c.result = &appData
	return c.result, nil
}

type cachedJob struct {
	cachedBase
	result      *jobTypes.Job
	cachedError error
}

type jobInfoResult struct {
	Job *jobTypes.Job `json:"job,omitempty"`
}

func (c *cachedJob) jobInfo(jobName string) (*jobTypes.Job, error) {
	c.Lock()
	defer c.Unlock()
	if c.result != nil {
		return c.result, nil
	}
	if c.cachedError != nil {
		return nil, c.cachedError
	}
	var jobInfo jobInfoResult
	err := c.cli.doRequest(http.MethodGet, "/jobs/"+jobName, &jobInfo)
	if err != nil {
		if httpErr, ok := errors.Cause(err).(*HTTPError); ok {
			if httpErr.StatusCode == http.StatusNotFound {
				c.cachedError = err
			}
		}
		return nil, err
	}
	if jobInfo.Job == nil {
		return nil, errors.Errorf("empty data for job %q", jobName)
	}
	c.result = jobInfo.Job
	return c.result, nil
}

type cachedPool struct {
	cachedBase
	result *pool.Pool
}

func (c *cachedPool) poolInfo(poolName string) (*pool.Pool, error) {
	c.Lock()
	defer c.Unlock()
	if c.result != nil {
		return c.result, nil
	}
	var pool pool.Pool
	err := c.cli.doRequest("GET", fmt.Sprintf("/pools/%s", poolName), &pool)
	if err != nil {
		return nil, err
	}
	if pool.Name == "" {
		return nil, errors.Errorf("pool %q not found", poolName)
	}
	c.result = &pool
	return c.result, nil
}

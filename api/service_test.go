// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/acl-api/api/types"
	"github.com/tsuru/acl-api/service"
)

type serviceMock struct {
	bindAppCall   []map[string]string
	bindJobCall   []map[string]string
	removeAppCall []map[string]string
	removeJobCall []map[string]string
	addRuleCall   []*types.ServiceRule
}

func (s *serviceMock) Create(instance types.ServiceInstance) error {
	return nil
}
func (s *serviceMock) Find(instanceName string) (types.ServiceInstance, error) {
	return types.ServiceInstance{}, nil
}
func (s *serviceMock) List() ([]types.ServiceInstance, error) {
	return nil, nil
}
func (s *serviceMock) Delete(instanceName string) error {
	return nil
}
func (s *serviceMock) AddRule(instanceName string, r *types.ServiceRule) ([]types.Rule, error) {
	r.RuleID = "fake-rule-id"
	s.addRuleCall = append(s.addRuleCall, r)
	return []types.Rule{
		{
			Source: types.RuleType{
				TsuruApp: &types.TsuruAppRule{
					AppName: instanceName,
				},
			},
			Destination: r.Destination,
		},
	}, nil

}
func (s *serviceMock) RemoveRule(instanceName string, ruleID string) error {
	return nil
}
func (s *serviceMock) AddApp(instanceName string, appName string) ([]types.Rule, error) {
	s.bindAppCall = append(s.bindAppCall, map[string]string{
		"instanceName": instanceName,
		"appName":      appName,
	})
	return []types.Rule{}, nil
}
func (s *serviceMock) RemoveApp(instanceName string, appName string) error {
	s.removeAppCall = append(s.removeAppCall, map[string]string{
		"instanceName": instanceName,
		"appName":      appName,
	})
	return nil
}

func (s *serviceMock) AddJob(instanceName string, jobName string) ([]types.Rule, error) {
	s.bindJobCall = append(s.bindJobCall, map[string]string{
		"instanceName": instanceName,
		"jobName":      jobName,
	})
	return []types.Rule{}, nil
}
func (s *serviceMock) RemoveJob(instanceName string, jobName string) error {
	s.removeJobCall = append(s.removeJobCall, map[string]string{
		"instanceName": instanceName,
		"jobName":      jobName,
	})
	return nil
}

func Test_serviceBindApp(t *testing.T) {
	mock := &serviceMock{}
	service.GetService = func() service.Service {
		return mock
	}
	e := echo.New()
	configHandlers(e)
	srv := httptest.NewServer(e.Server.Handler)
	defer srv.Close()

	body := strings.NewReader("app-name=myapp")
	req, err := http.NewRequest("POST", srv.URL+"/resources/testsvc/bind-app", body)
	require.Nil(t, err)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	rsp, err := http.DefaultClient.Do(req)
	require.Nil(t, err)
	rsp.Body.Close()
	assert.Equal(t, 200, rsp.StatusCode)
	assert.Equal(t, []map[string]string{
		{
			"instanceName": "testsvc",
			"appName":      "myapp",
		},
	}, mock.bindAppCall)
}
func Test_serviceUnbindApp(t *testing.T) {
	mock := &serviceMock{}
	service.GetService = func() service.Service {
		return mock
	}
	e := echo.New()
	configHandlers(e)
	srv := httptest.NewServer(e.Server.Handler)
	defer srv.Close()

	body := strings.NewReader("app-name=myapp")
	req, err := http.NewRequest("DELETE", srv.URL+"/resources/testsvc/bind-app", body)
	require.Nil(t, err)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	rsp, err := http.DefaultClient.Do(req)
	require.Nil(t, err)
	rsp.Body.Close()
	assert.Equal(t, 200, rsp.StatusCode)
	assert.Equal(t, []map[string]string{
		{
			"instanceName": "testsvc",
			"appName":      "myapp",
		},
	}, mock.removeAppCall)
}

func Test_serviceBindJob(t *testing.T) {
	mock := &serviceMock{}
	service.GetService = func() service.Service {
		return mock
	}
	e := echo.New()
	configHandlers(e)
	srv := httptest.NewServer(e.Server.Handler)
	defer srv.Close()

	body := strings.NewReader("job-name=myjob")
	req, err := http.NewRequest("POST", srv.URL+"/resources/testsvc/bind-job", body)
	require.Nil(t, err)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	rsp, err := http.DefaultClient.Do(req)
	require.Nil(t, err)
	rsp.Body.Close()
	assert.Equal(t, 200, rsp.StatusCode)
	assert.Equal(t, []map[string]string{
		{
			"instanceName": "testsvc",
			"jobName":      "myjob",
		},
	}, mock.bindJobCall)
}
func Test_serviceUnbindJob(t *testing.T) {
	mock := &serviceMock{}
	service.GetService = func() service.Service {
		return mock
	}
	e := echo.New()
	configHandlers(e)
	srv := httptest.NewServer(e.Server.Handler)
	defer srv.Close()

	body := strings.NewReader("job-name=myjob")
	req, err := http.NewRequest("DELETE", srv.URL+"/resources/testsvc/bind-job", body)
	require.Nil(t, err)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	rsp, err := http.DefaultClient.Do(req)
	require.Nil(t, err)
	rsp.Body.Close()
	assert.Equal(t, 200, rsp.StatusCode)
	assert.Equal(t, []map[string]string{
		{
			"instanceName": "testsvc",
			"jobName":      "myjob",
		},
	}, mock.removeJobCall)
}

func Test_serviceRuleAdd(t *testing.T) {
	mock := &serviceMock{}
	service.GetService = func() service.Service {
		return mock
	}
	e := echo.New()
	configHandlers(e)
	srv := httptest.NewServer(e.Server.Handler)
	defer srv.Close()

	body := strings.NewReader(`{
		"destination": {
			"TsuruApp": {
				"AppName": "myapp"
			}
		}
	}`)
	req, err := http.NewRequest("POST", srv.URL+"/resources/testsvc/rule", body)
	require.Nil(t, err)
	req.Header.Add("Content-Type", "application/json")
	rsp, err := http.DefaultClient.Do(req)
	require.Nil(t, err)
	defer rsp.Body.Close()
	if !assert.Equal(t, 200, rsp.StatusCode) {
		body, _ := ioutil.ReadAll(rsp.Body)
		assert.Fail(t, "body: "+string(body))
	}

	outputRule := &types.ServiceRule{}
	err = json.NewDecoder(rsp.Body).Decode(outputRule)
	require.NoError(t, err)

	assert.Equal(t, "fake-rule-id", outputRule.RuleID)
}

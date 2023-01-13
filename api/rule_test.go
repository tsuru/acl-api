// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/acl-api/api/types"
	"github.com/tsuru/acl-api/rule"
	"github.com/tsuru/acl-api/storage"
	_ "github.com/tsuru/acl-api/storage/mongodb"
)

func init() {
	resetViper()
}

func resetViper() {
	viper.Reset()
	viper.AutomaticEnv()
	storagePath := viper.GetString("storage")
	if storagePath == "" {
		storagePath = "mongodb://localhost"
	}
	viper.Set("storage", storagePath+"/acltest-pkg-api")
}

func Test_addRule(t *testing.T) {
	stor, err := storage.GetServiceStorage()
	require.Nil(t, err)
	clearer := stor.(interface {
		ClearAll()
	})
	t.Run("ok", func(t *testing.T) {
		clearer.ClearAll()
		e := setupEcho()
		srv := httptest.NewServer(e.Server.Handler)
		defer srv.Close()

		body := strings.NewReader(`{
			"source": {
				"tsuruapp": {
					"appname": "myapp1"
				}
			},
			"destination": {
				"externaldns": {
					"name": "a.b.com",
					"ports": [
						{
							"protocol": "tcp",
							"port": 8080
						}
					]
				}
			},
			"target": "accept",
			"metadata": {
				"meta-a": "a",
				"meta-b": "b"
			}
		}`)
		req, err := http.NewRequest("POST", srv.URL+"/rules", body)
		require.Nil(t, err)
		req.Header.Add("Content-Type", "application/json")

		rsp, err := http.DefaultClient.Do(req)
		require.Nil(t, err)
		defer rsp.Body.Close()

		bodyData, err := ioutil.ReadAll(rsp.Body)
		require.Nil(t, err)
		assert.Equal(t, 201, rsp.StatusCode)
		var result types.Rule
		err = json.Unmarshal(bodyData, &result)
		require.Nil(t, err)
		assert.NotEmpty(t, result.RuleID)
		assert.Equal(t, types.RuleType{
			TsuruApp: &types.TsuruAppRule{
				AppName: "myapp1",
			},
		}, result.Source)
		assert.Equal(t, types.RuleType{
			ExternalDNS: &types.ExternalDNSRule{
				Name: "a.b.com",
				Ports: []types.ProtoPort{
					{Port: 8080, Protocol: "tcp"},
				},
			},
		}, result.Destination)
		assert.Equal(t, map[string]string{
			"meta-a": "a",
			"meta-b": "b",
		}, result.Metadata)
	})

	t.Run("provided ruleName", func(t *testing.T) {
		clearer.ClearAll()
		e := setupEcho()
		srv := httptest.NewServer(e.Server.Handler)
		defer srv.Close()

		body := strings.NewReader(`{
			"ruleName": "my-rule-name",
			"source": {
				"tsuruapp": {
					"appname": "myapp1"
				}
			},
			"destination": {
				"externaldns": {
					"name": "a.b.com",
					"ports": [
						{
							"protocol": "tcp",
							"port": 8080
						}
					]
				}
			},
			"target": "accept",
			"metadata": {
				"meta-a": "a",
				"meta-b": "b"
			}
		}`)
		req, err := http.NewRequest("POST", srv.URL+"/rules", body)
		require.Nil(t, err)
		req.Header.Add("Content-Type", "application/json")

		rsp, err := http.DefaultClient.Do(req)
		require.Nil(t, err)
		defer rsp.Body.Close()

		assert.Equal(t, 201, rsp.StatusCode)
		var result types.Rule
		err = json.NewDecoder(rsp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Equal(t, "my-rule-name", result.RuleName)
		assert.Equal(t, types.RuleType{
			TsuruApp: &types.TsuruAppRule{
				AppName: "myapp1",
			},
		}, result.Source)
		assert.Equal(t, types.RuleType{
			ExternalDNS: &types.ExternalDNSRule{
				Name: "a.b.com",
				Ports: []types.ProtoPort{
					{Port: 8080, Protocol: "tcp"},
				},
			},
		}, result.Destination)
		assert.Equal(t, map[string]string{
			"meta-a": "a",
			"meta-b": "b",
		}, result.Metadata)
	})

	t.Run("duplicated name", func(t *testing.T) {
		clearer.ClearAll()
		e := setupEcho()
		srv := httptest.NewServer(e.Server.Handler)
		defer srv.Close()

		body := `{
			"ruleName": "my-rule-name",
			"source": {
				"tsuruapp": {
					"appname": "myapp1"
				}
			},
			"destination": {
				"externaldns": {
					"name": "a.b.com",
					"ports": [
						{
							"protocol": "tcp",
							"port": 8080
						}
					]
				}
			},
			"target": "accept",
			"metadata": {
				"meta-a": "a",
				"meta-b": "b"
			}
		}`
		req, err := http.NewRequest("POST", srv.URL+"/rules", strings.NewReader(body))
		require.Nil(t, err)
		req.Header.Add("Content-Type", "application/json")

		rsp, err := http.DefaultClient.Do(req)
		require.Nil(t, err)
		rsp.Body.Close()
		assert.Equal(t, 201, rsp.StatusCode)

		req, err = http.NewRequest("POST", srv.URL+"/rules", strings.NewReader(body))
		require.Nil(t, err)
		req.Header.Add("Content-Type", "application/json")

		rsp, err = http.DefaultClient.Do(req)
		require.Nil(t, err)
		defer rsp.Body.Close()
		assert.Equal(t, http.StatusConflict, rsp.StatusCode)

		responseBody := map[string]string{}
		err = json.NewDecoder(rsp.Body).Decode(&responseBody)
		require.NoError(t, err)

		assert.Equal(t, "RuleName: my-rule-name already in use", responseBody["message"])
	})

	t.Run("invalid name", func(t *testing.T) {
		clearer.ClearAll()
		e := setupEcho()
		srv := httptest.NewServer(e.Server.Handler)
		defer srv.Close()

		body := strings.NewReader(`{
			"ruleName": "MyInvalidRuleIDdddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd/ddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddMyInvalidRuleIDdddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd/dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			"source": {
				"tsuruapp": {
					"appname": "myapp1"
				}
			},
			"destination": {
				"externaldns": {
					"name": "a.b.com",
					"ports": [
						{
							"protocol": "tcp",
							"port": 8080
						}
					]
				}
			},
			"target": "accept",
			"metadata": {
				"meta-a": "a",
				"meta-b": "b"
			}
		}`)
		req, err := http.NewRequest("POST", srv.URL+"/rules", body)
		require.Nil(t, err)
		req.Header.Add("Content-Type", "application/json")

		rsp, err := http.DefaultClient.Do(req)
		require.Nil(t, err)
		defer rsp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, rsp.StatusCode)
		result := struct{ Message string }{}
		err = json.NewDecoder(rsp.Body).Decode(&result)
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(result.Message, "RuleName: must be no more than 253 characters"), "received message: "+result.Message)
	})

}

func Test_listRules(t *testing.T) {
	stor, err := storage.GetServiceStorage()
	require.Nil(t, err)
	clearer := stor.(interface {
		ClearAll()
	})
	clearer.ClearAll()
	svc := rule.GetService()
	err = svc.Save([]*types.Rule{
		{
			RuleID: "1",
			Source: types.RuleType{
				TsuruApp: &types.TsuruAppRule{
					AppName: "app1",
				},
			},
			Destination: types.RuleType{
				ExternalIP: &types.ExternalIPRule{
					IP: "192.168.90.0/24",
				},
			},
			Metadata: map[string]string{
				"meta-a": "a",
				"meta-b": "b",
			},
		},
	}, false)
	require.Nil(t, err)
	err = svc.Save([]*types.Rule{
		{
			RuleID: "2",
			Source: types.RuleType{
				TsuruApp: &types.TsuruAppRule{
					AppName: "app2",
				},
			},
			Destination: types.RuleType{
				ExternalIP: &types.ExternalIPRule{
					IP: "192.168.90.0/24",
				},
			},
			Metadata: map[string]string{
				"meta-a": "a",
				"meta-b": "c",
			},
		},
	}, false)
	require.Nil(t, err)
	t.Run("all", func(t *testing.T) {
		e := setupEcho()
		srv := httptest.NewServer(e.Server.Handler)
		defer srv.Close()

		req, err := http.NewRequest("GET", srv.URL+"/rules", nil)
		require.Nil(t, err)

		rsp, err := http.DefaultClient.Do(req)
		require.Nil(t, err)
		defer rsp.Body.Close()

		bodyData, err := ioutil.ReadAll(rsp.Body)
		require.Nil(t, err)
		assert.Equal(t, 200, rsp.StatusCode)
		var result []types.Rule
		err = json.Unmarshal(bodyData, &result)
		require.Nil(t, err)
		assert.Len(t, result, 2)
		assert.NotEmpty(t, result[0].Created)
		assert.NotEmpty(t, result[1].Created)
		result[0].Created = time.Time{}
		result[1].Created = time.Time{}
		sort.Slice(result, func(i, j int) bool {
			return result[i].RuleID < result[j].RuleID
		})
		assert.Equal(t, []types.Rule{
			{
				RuleID: "1",
				Source: types.RuleType{
					TsuruApp: &types.TsuruAppRule{
						AppName: "app1",
					},
				},
				Destination: types.RuleType{
					ExternalIP: &types.ExternalIPRule{
						IP:    "192.168.90.0/24",
						Ports: []types.ProtoPort{},
					},
				},
				Metadata: map[string]string{
					"meta-a": "a",
					"meta-b": "b",
				},
			},
			{
				RuleID: "2",
				Source: types.RuleType{
					TsuruApp: &types.TsuruAppRule{
						AppName: "app2",
					},
				},
				Destination: types.RuleType{
					ExternalIP: &types.ExternalIPRule{
						IP:    "192.168.90.0/24",
						Ports: []types.ProtoPort{},
					},
				},
				Metadata: map[string]string{
					"meta-a": "a",
					"meta-b": "c",
				},
			},
		}, result)
	})

	for _, tt := range []struct {
		url      string
		expected []string
	}{
		{url: "/rules?metadata.meta-b=c", expected: []string{"2"}},
		{url: "/rules?source.tsuruapp.appname=app1", expected: []string{"1"}},
		{url: "/rules?metadata.meta-a=a", expected: []string{"1", "2"}},
		{url: "/rules?metadata.meta-a=a&source.tsuruapp.appname=app2", expected: []string{"2"}},
	} {
		t.Run("filtered "+tt.url, func(t *testing.T) {
			e := setupEcho()
			srv := httptest.NewServer(e.Server.Handler)
			defer srv.Close()

			req, err := http.NewRequest("GET", srv.URL+tt.url, nil)
			require.Nil(t, err)

			rsp, err := http.DefaultClient.Do(req)
			require.Nil(t, err)
			defer rsp.Body.Close()

			bodyData, err := ioutil.ReadAll(rsp.Body)
			require.Nil(t, err)
			assert.Equal(t, 200, rsp.StatusCode)
			var result []types.Rule
			err = json.Unmarshal(bodyData, &result)
			require.Nil(t, err)
			var ruleIDs []string
			for _, r := range result {
				ruleIDs = append(ruleIDs, r.RuleID)
			}
			sort.Strings(ruleIDs)
			assert.Equal(t, ruleIDs, tt.expected)
		})
	}
}

func Test_getRule(t *testing.T) {
	stor, err := storage.GetServiceStorage()
	require.Nil(t, err)
	clearer := stor.(interface {
		ClearAll()
	})
	clearer.ClearAll()
	svc := rule.GetService()
	err = svc.Save([]*types.Rule{
		{
			RuleID:   "1",
			RuleName: "one",
			Source: types.RuleType{
				TsuruApp: &types.TsuruAppRule{
					AppName: "app1",
				},
			},
			Destination: types.RuleType{
				ExternalIP: &types.ExternalIPRule{
					IP: "192.168.90.0/24",
				},
			},
			Metadata: map[string]string{
				"meta-a": "a",
				"meta-b": "b",
			},
		},
	}, false)
	require.Nil(t, err)
	t.Run("ok", func(t *testing.T) {
		e := setupEcho()
		srv := httptest.NewServer(e.Server.Handler)
		defer srv.Close()

		req, err := http.NewRequest("GET", srv.URL+"/rules/1", nil)
		require.Nil(t, err)

		rsp, err := http.DefaultClient.Do(req)
		require.Nil(t, err)
		defer rsp.Body.Close()

		require.Equal(t, http.StatusOK, rsp.StatusCode)
		var result types.Rule
		err = json.NewDecoder(rsp.Body).Decode(&result)
		require.Nil(t, err)
		assert.NotEmpty(t, result.Created)
		result.Created = time.Time{}
		assert.Equal(t, types.Rule{
			RuleID:   "1",
			RuleName: "one",
			Source: types.RuleType{
				TsuruApp: &types.TsuruAppRule{
					AppName: "app1",
				},
			},
			Destination: types.RuleType{
				ExternalIP: &types.ExternalIPRule{
					IP:    "192.168.90.0/24",
					Ports: []types.ProtoPort{},
				},
			},
			Metadata: map[string]string{
				"meta-a": "a",
				"meta-b": "b",
			},
		}, result)
	})

	t.Run("using ruleName", func(t *testing.T) {
		e := setupEcho()
		srv := httptest.NewServer(e.Server.Handler)
		defer srv.Close()

		req, err := http.NewRequest("GET", srv.URL+"/rules/one", nil)
		require.Nil(t, err)

		rsp, err := http.DefaultClient.Do(req)
		require.Nil(t, err)
		defer rsp.Body.Close()

		require.Equal(t, http.StatusOK, rsp.StatusCode)

		var result types.Rule
		err = json.NewDecoder(rsp.Body).Decode(&result)
		require.Nil(t, err)
		assert.NotEmpty(t, result.Created)
		result.Created = time.Time{}
		assert.Equal(t, types.Rule{
			RuleID:   "1",
			RuleName: "one",
			Source: types.RuleType{
				TsuruApp: &types.TsuruAppRule{
					AppName: "app1",
				},
			},
			Destination: types.RuleType{
				ExternalIP: &types.ExternalIPRule{
					IP:    "192.168.90.0/24",
					Ports: []types.ProtoPort{},
				},
			},
			Metadata: map[string]string{
				"meta-a": "a",
				"meta-b": "b",
			},
		}, result)
	})
	t.Run("not found", func(t *testing.T) {
		e := setupEcho()
		srv := httptest.NewServer(e.Server.Handler)
		defer srv.Close()

		req, err := http.NewRequest("GET", srv.URL+"/rules/2", nil)
		require.Nil(t, err)

		rsp, err := http.DefaultClient.Do(req)
		require.Nil(t, err)
		defer rsp.Body.Close()

		assert.Equal(t, 404, rsp.StatusCode)
	})
	t.Run("invalid", func(t *testing.T) {
		e := setupEcho()
		srv := httptest.NewServer(e.Server.Handler)
		defer srv.Close()

		req, err := http.NewRequest("GET", srv.URL+"/rules/  ", nil)
		require.Nil(t, err)

		rsp, err := http.DefaultClient.Do(req)
		require.Nil(t, err)
		defer rsp.Body.Close()

		assert.Equal(t, 400, rsp.StatusCode)
	})
}

func Test_deleteRule(t *testing.T) {
	stor, err := storage.GetServiceStorage()
	require.Nil(t, err)
	clearer := stor.(interface {
		ClearAll()
	})
	createRule := func() {
		clearer.ClearAll()
		svc := rule.GetService()
		err = svc.Save([]*types.Rule{
			{
				RuleID: "1",
				Source: types.RuleType{
					TsuruApp: &types.TsuruAppRule{
						AppName: "app1",
					},
				},
				Destination: types.RuleType{
					ExternalIP: &types.ExternalIPRule{
						IP: "192.168.90.0/24",
					},
				},
				Metadata: map[string]string{
					"meta-a": "a",
					"meta-b": "b",
				},
			},
		}, false)
	}
	require.Nil(t, err)
	t.Run("ok", func(t *testing.T) {
		createRule()
		e := setupEcho()
		srv := httptest.NewServer(e.Server.Handler)
		defer srv.Close()

		req, err := http.NewRequest("DELETE", srv.URL+"/rules/1", nil)
		require.Nil(t, err)

		rsp, err := http.DefaultClient.Do(req)
		require.Nil(t, err)
		defer rsp.Body.Close()

		assert.Equal(t, 200, rsp.StatusCode)

		req, err = http.NewRequest("DELETE", srv.URL+"/rules/1", nil)
		require.Nil(t, err)

		rsp, err = http.DefaultClient.Do(req)
		require.Nil(t, err)
		defer rsp.Body.Close()

		assert.Equal(t, 404, rsp.StatusCode)
	})
	t.Run("not found", func(t *testing.T) {
		createRule()
		e := setupEcho()
		srv := httptest.NewServer(e.Server.Handler)
		defer srv.Close()

		req, err := http.NewRequest("DELETE", srv.URL+"/rules/2", nil)
		require.Nil(t, err)

		rsp, err := http.DefaultClient.Do(req)
		require.Nil(t, err)
		defer rsp.Body.Close()

		assert.Equal(t, 404, rsp.StatusCode)
	})
	t.Run("invalid", func(t *testing.T) {
		createRule()
		e := setupEcho()
		srv := httptest.NewServer(e.Server.Handler)
		defer srv.Close()

		req, err := http.NewRequest("DELETE", srv.URL+"/rules/  ", nil)
		require.Nil(t, err)

		rsp, err := http.DefaultClient.Do(req)
		require.Nil(t, err)
		defer rsp.Body.Close()

		assert.Equal(t, 400, rsp.StatusCode)
	})
}

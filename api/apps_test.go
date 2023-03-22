package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/acl-api/api/types"
	"github.com/tsuru/acl-api/rule"
	"github.com/tsuru/acl-api/storage"
)

func Test_getAppsRules(t *testing.T) {
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
		{
			RuleID:   "3",
			RuleName: "three",
			Source: types.RuleType{
				TsuruApp: &types.TsuruAppRule{
					AppName: "app3",
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

		req, err := http.NewRequest("GET", srv.URL+"/apps/app1/rules", nil)
		require.Nil(t, err)

		rsp, err := http.DefaultClient.Do(req)
		require.Nil(t, err)
		defer rsp.Body.Close()

		require.Equal(t, http.StatusOK, rsp.StatusCode)
		var result []types.Rule
		err = json.NewDecoder(rsp.Body).Decode(&result)
		require.Nil(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "1", result[0].RuleID)
	})

	t.Run("empty", func(t *testing.T) {
		e := setupEcho()
		srv := httptest.NewServer(e.Server.Handler)
		defer srv.Close()

		req, err := http.NewRequest("GET", srv.URL+"/apps/app2/rules", nil)
		require.Nil(t, err)

		rsp, err := http.DefaultClient.Do(req)
		require.Nil(t, err)
		defer rsp.Body.Close()

		require.Equal(t, http.StatusOK, rsp.StatusCode)
		var result []types.Rule
		err = json.NewDecoder(rsp.Body).Decode(&result)
		require.Nil(t, err)
		require.Len(t, result, 0)
	})
}

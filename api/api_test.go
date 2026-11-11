// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/acl-api/api/version"
)

func TestAuthentication(t *testing.T) {

	tests := []struct {
		name          string
		authorization string
		config        map[string]string
		expectedCode  int
		method        string
	}{
		{
			name:         "no authentication, no headers",
			expectedCode: 200,
		},
		{
			name: "basic auth, no headers",
			config: map[string]string{
				"auth.user":     "admin",
				"auth.password": "admin",
			},
			expectedCode: 401,
		},
		{
			name:          "basic auth, with headers",
			authorization: "basic " + base64.StdEncoding.EncodeToString([]byte("admin:admin")),
			config: map[string]string{
				"auth.user":     "admin",
				"auth.password": "admin",
			},
			expectedCode: 200,
		},

		{
			name:          "basic auth, with headers, read only user",
			authorization: "basic " + base64.StdEncoding.EncodeToString([]byte("guest:guest")),
			config: map[string]string{
				"auth.read_only_user":     "guest",
				"auth.read_only_password": "guest",
			},
			method:       "GET",
			expectedCode: 200,
		},

		{
			name:          "basic auth, with headers, read only user, non GET URL",
			authorization: "basic " + base64.StdEncoding.EncodeToString([]byte("guest:guest")),
			config: map[string]string{
				"auth.read_only_user":     "guest",
				"auth.read_only_password": "guest",
			},
			method:       "POST",
			expectedCode: 401,
		},
	}

	e := setupEcho()
	e.Any("/test1", func(c echo.Context) error {
		c.String(200, "ok")
		return nil
	})
	srv := httptest.NewServer(e.Server.Handler)
	defer srv.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer resetViper()
			for k, v := range tt.config {
				viper.Set(k, v)
			}

			if tt.method == "" {
				tt.method = "GET"
			}
			req, err := http.NewRequest(tt.method, srv.URL+"/test1", nil)
			require.Nil(t, err)
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}
			rsp, err := http.DefaultClient.Do(req)
			require.Nil(t, err)
			defer rsp.Body.Close()
			assert.Equal(t, tt.expectedCode, rsp.StatusCode)
		})
	}
}

func TestSwaggerDocumentationIsPublic(t *testing.T) {
	defer resetViper()
	viper.Set("auth.user", "admin")
	viper.Set("auth.password", "admin")

	e := setupEcho()
	srv := httptest.NewServer(e.Server.Handler)
	defer srv.Close()

	for _, path := range []string{"/swagger/index.html", "/swagger/doc.json", "/swagger/doc.yaml"} {
		t.Run(path, func(t *testing.T) {
			response, err := http.Get(srv.URL + path)
			require.NoError(t, err)
			defer response.Body.Close()
			assert.Equal(t, http.StatusOK, response.StatusCode)
		})
	}

	response, err := http.Get(srv.URL + "/swagger/doc.json")
	require.NoError(t, err)
	defer response.Body.Close()

	var document struct {
		Swagger string `json:"swagger"`
		Info    struct {
			Version string `json:"version"`
		} `json:"info"`
		Paths map[string]json.RawMessage `json:"paths"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&document))
	assert.Equal(t, "2.0", document.Swagger)
	assert.Equal(t, version.Version, document.Info.Version)
	assert.Contains(t, document.Paths, "/rules")
	assert.Contains(t, document.Paths, "/resources/{instance}/rule")

	response, err = http.Get(srv.URL + "/rules")
	require.NoError(t, err)
	defer response.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
	assert.True(t, strings.HasPrefix(response.Header.Get("WWW-Authenticate"), "Basic"))
}

func TestHTTPErrorWithInternalError(t *testing.T) {
	e := setupEcho()
	e.GET("/test-error", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusBadRequest, "bad request").SetInternal(errors.New("underlying failure"))
	})
	recorder := httptest.NewRecorder()
	e.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/test-error", nil))
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	var body struct {
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, "code=400, message=bad request, internal=underlying failure", body.Message)
}

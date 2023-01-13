// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

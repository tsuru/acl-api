// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/acl-api/engine"
	"github.com/tsuru/acl-api/rule"
)

// appForceSyncRule synchronizes every rule sourced from an application.
// @Summary Synchronize application rules
// @Tags applications
// @Produce json
// @Param app path string true "Application name"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200 {object} SyncCountResponse
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /apps/{app}/sync [post]
func appForceSyncRule(c echo.Context) error {
	app := c.Param("app")
	rulesSvc := rule.GetService()

	rules, err := rulesSvc.FindBySourceTsuruApp(app)

	if err != nil {
		return err
	}

	engine.SyncRules(rules, true)

	return c.JSON(http.StatusOK, map[string]int{"count": len(rules)})
}

// appRules lists rules sourced from an application.
// @Summary List application rules
// @Tags applications
// @Produce json
// @Param app path string true "Application name"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200 {array} types.Rule
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /apps/{app}/rules [get]
func appRules(c echo.Context) error {
	app := c.Param("app")
	rulesSvc := rule.GetService()

	rules, err := rulesSvc.FindBySourceTsuruApp(app)

	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, rules)
}

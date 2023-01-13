// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"net/http"

	"github.com/labstack/echo"
	"github.com/tsuru/acl-api/engine"
	"github.com/tsuru/acl-api/rule"
)

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

func appRules(c echo.Context) error {
	app := c.Param("app")
	rulesSvc := rule.GetService()

	rules, err := rulesSvc.FindBySourceTsuruApp(app)

	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, rules)
}

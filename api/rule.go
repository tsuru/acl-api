// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ajg/form"
	"github.com/labstack/echo"
	"github.com/tsuru/acl-api/api/types"
	"github.com/tsuru/acl-api/engine"
	"github.com/tsuru/acl-api/rule"
	"github.com/tsuru/acl-api/storage"
	"k8s.io/apimachinery/pkg/util/validation"
)

func listRules(c echo.Context) error {
	var filter types.Rule
	d := form.NewDecoder(nil)
	d.IgnoreCase(true)
	d.IgnoreUnknownKeys(true)
	err := d.DecodeValues(&filter, c.QueryParams())
	if err != nil {
		return err
	}
	svc := rule.GetService()
	rules, err := svc.FindByRule(filter)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, rules)
}

func latestSync(c echo.Context) error {
	rulesSvc := rule.GetService()
	rulesSyncs, err := rulesSvc.FindSyncs(nil)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, rulesSyncs)
}

func addRule(c echo.Context) error {
	var r types.Rule
	err := c.Bind(&r)
	if err != nil {
		return err
	}
	r.RuleID = ""
	if r.RuleName != "" {
		errs := validation.IsDNS1123Subdomain(r.RuleName)
		if len(errs) > 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "RuleName: "+strings.Join(errs, "\n"))
		}
	}
	r.Created = time.Time{}
	if user := c.Get("user"); user != nil {
		r.Creator = fmt.Sprint(user)
	}
	svc := rule.GetService()
	err = svc.Save([]*types.Rule{&r}, false)
	if err == storage.ErrInstanceAlreadyExists {
		return echo.NewHTTPError(http.StatusConflict, "RuleName: "+r.RuleName+" already in use")
	}

	if err != nil {
		return err
	}
	waitSync, _ := strconv.ParseBool(c.FormValue("wait-sync"))
	if waitSync {
		engine.SyncRules([]types.Rule{r}, false)
	} else {
		go engine.SyncRules([]types.Rule{r}, false)
	}
	return c.JSON(http.StatusCreated, r)
}

func deleteRule(c echo.Context) error {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "empty rule id")
	}
	svc := rule.GetService()
	err := svc.Delete(id)
	if err == storage.ErrRuleNotFound {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	return err
}

func getRule(c echo.Context) error {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "empty rule id")
	}
	svc := rule.GetService()
	rule, err := svc.FindByID(id)
	if err == storage.ErrRuleNotFound {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, rule)
}

func forceRuleSync(c echo.Context) error {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "empty rule id")
	}
	svc := rule.GetService()
	rule, err := svc.FindByID(id)
	if err == storage.ErrRuleNotFound {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	if err != nil {
		return err
	}
	engine.SyncRules([]types.Rule{rule}, true)
	return nil
}

func getRuleSync(c echo.Context) error {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "empty rule id")
	}
	rulesSvc := rule.GetService()
	rulesSyncs, err := rulesSvc.FindSyncs([]string{id})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, rulesSyncs)
}

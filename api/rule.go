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
	"github.com/labstack/echo/v4"
	"github.com/tsuru/acl-api/api/types"
	"github.com/tsuru/acl-api/engine"
	"github.com/tsuru/acl-api/rule"
	"github.com/tsuru/acl-api/storage"
	"k8s.io/apimachinery/pkg/util/validation"
)

// listRules lists rules, optionally filtered by fields accepted in the query string.
// @Summary List rules
// @Tags rules
// @Produce json
// @Param RuleID query string false "Rule ID"
// @Param RuleName query string false "Rule name"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200 {array} types.Rule
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /rules [get]
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

// latestSync lists the latest synchronization data for all rules.
// @Summary List rule synchronization data
// @Tags rules
// @Produce json
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200 {array} types.RuleSyncInfo
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /rules/sync [get]
func latestSync(c echo.Context) error {
	rulesSvc := rule.GetService()
	rulesSyncs, err := rulesSvc.FindSyncs(nil)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, rulesSyncs)
}

// addRule creates a rule and schedules its synchronization.
// @Summary Create a rule
// @Tags rules
// @Accept json
// @Produce json
// @Param rule body types.Rule true "Rule"
// @Param wait-sync query boolean false "Wait for synchronization before responding"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 201 {object} types.Rule
// @Failure 400 {object} APIError
// @Failure 409 {object} APIError
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /rules [post]
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

// deleteRule deletes a rule by ID.
// @Summary Delete a rule
// @Tags rules
// @Param id path string true "Rule ID"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200
// @Failure 400 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /rules/{id} [delete]
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

// getRule returns one rule by ID.
// @Summary Get a rule
// @Tags rules
// @Produce json
// @Param id path string true "Rule ID"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200 {object} types.Rule
// @Failure 400 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /rules/{id} [get]
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

// forceRuleSync synchronizes a rule immediately.
// @Summary Synchronize a rule
// @Tags rules
// @Param id path string true "Rule ID"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200
// @Failure 400 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /rules/{id}/sync [post]
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

// getRuleSync lists synchronization data for one rule.
// @Summary Get rule synchronization data
// @Tags rules
// @Produce json
// @Param id path string true "Rule ID"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200 {array} types.RuleSyncInfo
// @Failure 400 {object} APIError
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /rules/{id}/sync [get]
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

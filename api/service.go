// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo"
	"github.com/tsuru/acl-api/api/types"
	"github.com/tsuru/acl-api/engine"
	"github.com/tsuru/acl-api/rule"
	"github.com/tsuru/acl-api/service"
)

func serviceCreate(c echo.Context) error {
	var instance types.ServiceInstance
	instance.InstanceName = c.FormValue("name")
	instance.Creator = c.FormValue("user")
	instance.EventID = c.FormValue("eventid")
	svc := service.GetService()
	err := svc.Create(instance)
	if err != nil {
		return err
	}
	return c.String(http.StatusOK, "")
}

func serviceDelete(c echo.Context) error {
	instanceName := c.Param("instance")
	svc := service.GetService()
	err := svc.Delete(instanceName)
	if err != nil {
		return err
	}
	return c.String(http.StatusOK, "")
}

func serviceStatus(c echo.Context) error {
	return nil
}

type infoItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

func serviceInfo(c echo.Context) error {
	instanceName := c.Param("instance")
	svc := service.GetService()
	si, err := svc.Find(instanceName)
	if err != nil {
		return err
	}
	var rulesStr []string
	for _, r := range si.BaseRules {
		val := fmt.Sprintf("Rule ID: %s - Destination: %s", r.RuleID, r.Destination.String())
		rulesStr = append(rulesStr, val)
	}
	item := infoItem{
		Label: "Rules",
		Value: strings.Join(rulesStr, "\n"),
	}
	return c.JSON(http.StatusOK, []infoItem{item})
}

func serviceBindApp(c echo.Context) error {
	instanceName := c.Param("instance")
	appName := c.FormValue("app-name")
	if appName == "" {
		c.String(http.StatusBadRequest, "app-name is required")
	}
	svc := service.GetService()
	rules, err := svc.AddApp(instanceName, appName)
	if err != nil {
		return err
	}
	go engine.SyncRules(rules, false)
	return c.JSON(http.StatusOK, map[string]string{})
}

func serviceUnbindApp(c echo.Context) error {
	req := c.Request()
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	query, err := url.ParseQuery(string(data))
	if err != nil {
		return err
	}
	instanceName := c.Param("instance")
	appName := query.Get("app-name")
	if appName == "" {
		c.String(http.StatusBadRequest, "app-name is required")
	}
	svc := service.GetService()
	err = svc.RemoveApp(instanceName, appName)
	if err != nil {
		return err
	}
	return c.String(http.StatusOK, "")
}

func serviceBindJob(c echo.Context) error {
	instanceName := c.Param("instance")
	jobName := c.FormValue("job-name")
	if jobName == "" {
		c.String(http.StatusBadRequest, "job-name is required")
	}
	svc := service.GetService()
	rules, err := svc.AddJob(instanceName, jobName)
	if err != nil {
		return err
	}
	go engine.SyncRules(rules, false)
	return c.JSON(http.StatusOK, map[string]string{})
}

func serviceUnbindJob(c echo.Context) error {
	req := c.Request()
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	query, err := url.ParseQuery(string(data))
	if err != nil {
		return err
	}
	instanceName := c.Param("instance")
	jobName := query.Get("job-name")
	if jobName == "" {
		c.String(http.StatusBadRequest, "job-name is required")
	}
	svc := service.GetService()
	err = svc.RemoveJob(instanceName, jobName)
	if err != nil {
		return err
	}
	return c.String(http.StatusOK, "")
}

func serviceBindUnit(c echo.Context) error {
	// noop
	return nil
}

func serviceUnbindUnit(c echo.Context) error {
	// noop
	return nil
}

func listServices(c echo.Context) error {
	svc := service.GetService()
	sis, err := svc.List()
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, sis)
}

type serviceRuleData struct {
	ServiceInstance types.ServiceInstance
	ExpandedRules   []types.Rule
	RulesSync       []types.RuleSyncInfo
}

func serviceListRules(c echo.Context) error {
	instanceName := c.Param("instance")
	svc := service.GetService()
	si, err := svc.Find(instanceName)
	if err != nil {
		return err
	}
	rulesSvc := rule.GetService()
	rules, err := rulesSvc.FindMetadata(map[string]string{
		"owner":         service.OwnerAclFromHell,
		"instance-name": instanceName,
	})
	if err != nil {
		return err
	}
	ruleIDs := make([]string, len(rules))
	for i, r := range rules {
		ruleIDs[i] = r.RuleID
	}
	rulesSync, err := rulesSvc.FindSyncs(ruleIDs)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, serviceRuleData{
		ServiceInstance: si,
		ExpandedRules:   rules,
		RulesSync:       rulesSync,
	})
}

func serviceAddRule(c echo.Context) error {
	instanceName := c.Param("instance")
	r := &types.ServiceRule{}
	err := c.Bind(r)
	if err != nil {
		return err
	}
	r.RuleID = ""
	r.Created = time.Time{}
	r.Creator = c.Request().Header.Get("X-Tsuru-User")
	r.EventID = c.Request().Header.Get("X-Tsuru-Eventid")

	err = r.Destination.Validate()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	svc := service.GetService()
	rules, err := svc.AddRule(instanceName, r)
	if err == service.ErrRuleAlreadyExists {
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	}
	if err != nil {
		return err
	}
	go engine.SyncRules(rules, false)
	return c.JSON(http.StatusOK, r)
}

func serviceRemoveRule(c echo.Context) error {
	instanceName := c.Param("instance")
	ruleID := c.Param("rule")
	svc := service.GetService()
	err := svc.RemoveRule(instanceName, ruleID)
	if err != nil {
		return err
	}
	return c.String(http.StatusOK, "")
}

func serviceForceSyncRule(c echo.Context) error {
	instanceName := c.Param("instance")
	rulesSvc := rule.GetService()

	rules, err := rulesSvc.FindMetadata(map[string]string{
		"instance-name": instanceName,
	})

	if err != nil {
		return err
	}

	engine.SyncRules(rules, true)

	return nil
}

func servicePlans(c echo.Context) error {
	return c.JSONBlob(http.StatusOK, []byte("[]"))
}

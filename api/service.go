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

	"github.com/labstack/echo/v4"
	"github.com/tsuru/acl-api/api/types"
	"github.com/tsuru/acl-api/engine"
	"github.com/tsuru/acl-api/rule"
	"github.com/tsuru/acl-api/service"
	"github.com/tsuru/acl-api/storage"
)

// serviceCreate creates a Tsuru service instance.
// @Summary Create a service instance
// @Tags service-resources
// @Accept application/x-www-form-urlencoded
// @Param name formData string true "Instance name"
// @Param user formData string false "Creator"
// @Param eventid formData string false "Tsuru event ID"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /resources [post]
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

// serviceUpdate checks that a service instance exists; it has no update behavior.
// @Summary Update a service instance
// @Tags service-resources
// @Param instance path string true "Instance name"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200
// @Failure 404
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /resources/{instance} [put]
func serviceUpdate(c echo.Context) error {
	// serviceUpdate is a no-op operation
	// just check if service exists
	instanceName := c.Param("instance")

	svc := service.GetService()
	_, err := svc.Find(instanceName)

	if err == storage.ErrInstanceNotFound {
		return c.String(http.StatusNotFound, "")
	}

	if err != nil {
		return err
	}

	return c.String(http.StatusOK, "")
}

// serviceDelete removes a service instance.
// @Summary Delete a service instance
// @Tags service-resources
// @Param instance path string true "Instance name"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /resources/{instance} [delete]
func serviceDelete(c echo.Context) error {
	instanceName := c.Param("instance")
	svc := service.GetService()
	err := svc.Delete(instanceName)
	if err != nil {
		return err
	}
	return c.String(http.StatusOK, "")
}

// serviceStatus returns the service-broker status response.
// @Summary Get service instance status
// @Tags service-resources
// @Param instance path string true "Instance name"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200
// @Security BasicAuth
// @Router /resources/{instance}/status [get]
func serviceStatus(c echo.Context) error {
	return nil
}

type infoItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// serviceInfo returns human-readable information about an instance.
// @Summary Get service instance information
// @Tags service-resources
// @Produce json
// @Param instance path string true "Instance name"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200 {array} infoItem
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /resources/{instance} [get]
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

// serviceBindApp binds a service instance to an application.
// @Summary Bind an application
// @Tags service-resources
// @Accept application/x-www-form-urlencoded
// @Produce json
// @Param instance path string true "Instance name"
// @Param app-name formData string true "Application name"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200 {object} EmptyResponse
// @Failure 400 {object} APIError
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /resources/{instance}/bind-app [post]
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

// serviceUnbindApp removes an application binding.
// @Summary Unbind an application
// @Tags service-resources
// @Accept application/x-www-form-urlencoded
// @Param instance path string true "Instance name"
// @Param app-name formData string true "Application name"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200
// @Failure 400 {object} APIError
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /resources/{instance}/bind-app [delete]
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

// serviceBindJob binds a service instance to a job.
// @Summary Bind a job
// @Tags service-resources
// @Produce json
// @Param instance path string true "Instance name"
// @Param job path string true "Job name"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200 {object} EmptyResponse
// @Failure 400 {object} APIError
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /resources/{instance}/binds/jobs/{job} [put]
func serviceBindJob(c echo.Context) error {
	instanceName := c.Param("instance")
	jobName := c.Param("job")
	if jobName == "" {
		c.String(http.StatusBadRequest, "job is required")
	}
	svc := service.GetService()
	rules, err := svc.AddJob(instanceName, jobName)
	if err != nil {
		return err
	}
	go engine.SyncRules(rules, false)
	return c.JSON(http.StatusOK, map[string]string{})
}

// serviceUnbindJob removes a job binding.
// @Summary Unbind a job
// @Tags service-resources
// @Param instance path string true "Instance name"
// @Param job path string true "Job name"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200
// @Failure 400 {object} APIError
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /resources/{instance}/binds/jobs/{job} [delete]
func serviceUnbindJob(c echo.Context) error {
	jobName := c.Param("job")
	instanceName := c.Param("instance")

	if jobName == "" {
		c.String(http.StatusBadRequest, "job-name is required")
	}
	svc := service.GetService()
	err := svc.RemoveJob(instanceName, jobName)
	if err != nil {
		return err
	}
	return c.String(http.StatusOK, "")
}

// serviceBindUnit is a no-op required by the Tsuru service-broker contract.
// @Summary Bind a unit
// @Tags service-resources
// @Param instance path string true "Instance name"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200
// @Security BasicAuth
// @Router /resources/{instance}/bind [post]
func serviceBindUnit(c echo.Context) error {
	// noop
	return nil
}

// serviceUnbindUnit is a no-op required by the Tsuru service-broker contract.
// @Summary Unbind a unit
// @Tags service-resources
// @Param instance path string true "Instance name"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200
// @Security BasicAuth
// @Router /resources/{instance}/bind [delete]
func serviceUnbindUnit(c echo.Context) error {
	// noop
	return nil
}

// listServices lists service instances.
// @Summary List service instances
// @Tags service-resources
// @Produce json
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200 {array} types.ServiceInstance
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /services [get]
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

// serviceListRules returns the instance, its expanded rules, and their synchronization data.
// @Summary List service instance rules
// @Tags service-resources
// @Produce json
// @Param instance path string true "Instance name"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200 {object} serviceRuleData
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /resources/{instance}/rule [get]
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

// serviceAddRule adds a rule to a service instance.
// @Summary Add a service instance rule
// @Tags service-resources
// @Accept json
// @Produce json
// @Param instance path string true "Instance name"
// @Param rule body types.ServiceRule true "Service rule"
// @Param X-Request-ID header string false "Request correlation ID"
// @Param X-Tsuru-User header string false "Tsuru user"
// @Param X-Tsuru-Eventid header string false "Tsuru event ID"
// @Success 200 {object} types.ServiceRule
// @Failure 400 {object} APIError
// @Failure 409 {object} APIError
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /resources/{instance}/rule [post]
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

// serviceRemoveRule removes a rule from a service instance.
// @Summary Remove a service instance rule
// @Tags service-resources
// @Param instance path string true "Instance name"
// @Param rule path string true "Rule ID"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /resources/{instance}/rule/{rule} [delete]
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

// serviceForceSyncRule synchronizes all rules for a service instance.
// @Summary Synchronize service instance rules
// @Tags service-resources
// @Param instance path string true "Instance name"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /resources/{instance}/sync [post]
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

// servicePlans lists service plans; ACL API currently has no plans.
// @Summary List service plans
// @Tags service-resources
// @Produce json
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200 {array} EmptyResponse
// @Security BasicAuth
// @Router /resources/plans [get]
func servicePlans(c echo.Context) error {
	return c.JSONBlob(http.StatusOK, []byte("[]"))
}

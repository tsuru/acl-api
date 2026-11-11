// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tsuru/acl-api/rule"
)

// jobRules lists rules sourced from a Tsuru job.
// @Summary List job rules
// @Tags jobs
// @Produce json
// @Param job path string true "Job name"
// @Param X-Request-ID header string false "Request correlation ID"
// @Success 200 {array} types.Rule
// @Failure 500 {object} APIError
// @Security BasicAuth
// @Router /jobs/{job}/rules [get]
func jobRules(c echo.Context) error {
	job := c.Param("job")
	rulesSvc := rule.GetService()

	rules, err := rulesSvc.FindBySourceTsuruJob(job)

	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, rules)
}

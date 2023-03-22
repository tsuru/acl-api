// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"net/http"

	"github.com/labstack/echo"
	"github.com/tsuru/acl-api/rule"
)

func jobRules(c echo.Context) error {
	job := c.Param("job")
	rulesSvc := rule.GetService()

	rules, err := rulesSvc.FindBySourceTsuruJob(job)

	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, rules)
}

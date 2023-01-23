// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/gops/agent"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/tsuru/acl-api/api/version"
	"github.com/tsuru/acl-api/engine"
	"github.com/tsuru/acl-api/engine/operator"
	_ "github.com/tsuru/acl-api/storage/mongodb"
)

func handleSignals(fn func()) {
	quit := make(chan os.Signal, 2)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	fn()
}

func shutdownEcho(e *echo.Echo) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		logrus.Fatal(err)
	}
}

func shutdownEngine() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if err := engine.ShutdownPeriodicSync(ctx); err != nil {
		logrus.Errorf("unable to shutdown periodic sync: %v", err)
	}
}

func shouldSkipAuth(path string) bool {
	return strings.HasPrefix(path, "/plugin") || path == "/healthcheck" || path == "/metrics"
}

type PluginManifest struct {
	SchemaVersion  string
	Metadata       PluginManifestMetadata
	URLPerPlatform map[string]string
}
type PluginManifestMetadata struct {
	Name    string
	Version string
}

func setupEcho() *echo.Echo {
	e := echo.New()
	e.Use(middleware.Logger())

	e.Use(middleware.BasicAuthWithConfig(middleware.BasicAuthConfig{
		Skipper: func(c echo.Context) bool {
			if skip, _ := c.Get("skip-basic-auth").(bool); skip {
				return true
			}
			if viper.GetString("auth.user") == "" &&
				viper.GetString("auth.password") == "" &&
				viper.GetString("auth.read_only_user") == "" &&
				viper.GetString("auth.read_only_password") == "" {
				return true
			}
			return shouldSkipAuth(c.Path())
		},
		Realm: "Restricted",
		Validator: func(username, password string, c echo.Context) (bool, error) {
			configUser := viper.GetString("auth.user")
			configPassword := viper.GetString("auth.password")
			if username == configUser && password == configPassword {
				c.Set("user", username)
				return true, nil
			}

			if c.Request().Method == http.MethodGet {
				configUser = viper.GetString("auth.read_only_user")
				configPassword = viper.GetString("auth.read_only_password")
				if configUser != "" && username == configUser && password == configPassword {
					c.Set("user", username)
					return true, nil
				}
			}

			return false, nil
		},
	}))

	e.Use(openTracingMiddleware)
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set(version.VersionHeader, version.Version)
			return next(c)
		}
	})

	e.HTTPErrorHandler = func(err error, c echo.Context) {
		var (
			code = http.StatusInternalServerError
			msg  interface{}
		)

		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
			msg = he.Message
			if he.Inner != nil {
				msg = fmt.Sprintf("%v, %v", err, he.Inner)
			}
		} else {
			msg = err.Error()
		}
		if _, ok := msg.(string); ok {
			msg = echo.Map{"message": msg}
		}

		e.Logger.Error(err)

		if !c.Response().Committed {
			if c.Request().Method == http.MethodHead {
				err = c.NoContent(code)
			} else {
				err = c.JSON(code, msg)
			}
			if err != nil {
				e.Logger.Error(err)
			}
		}
	}

	configHandlers(e)
	return e
}

var allEngines = []func() engine.Engine{
	func() engine.Engine {
		return &operator.ACLOperatorEngine{}
	},
}

func setupEngine() {
	enabledEngines := viper.GetStringSlice("engines")
	for _, engineName := range enabledEngines {
		for _, e := range allEngines {
			if e().Name() == engineName {
				engine.EnableEngine(e)
			}
		}
	}
}

func StartAPI() error {
	if err := agent.Listen(agent.Options{}); err != nil {
		return err
	}
	defer agent.Close()

	setupEngine()
	go engine.RunPeriodicSync()

	e := setupEcho()
	go handleSignals(func() {
		shutdownEcho(e)
	})

	err := e.Start(fmt.Sprintf(":%d", viper.GetInt("port")))
	logrus.Infof("Shutting down server: %v", err)
	shutdownEngine()
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func configHandlers(e *echo.Echo) {
	e.GET("/debug/pprof/*", echo.WrapHandler(http.DefaultServeMux))
	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
	e.GET("/rules", listRules)
	e.POST("/rules/:id/sync", forceRuleSync)
	e.POST("/rules", addRule)
	e.GET("/rules/:id/sync", getRuleSync)
	e.GET("/rules/:id", getRule)
	e.DELETE("/rules/:id", deleteRule)
	e.GET("/rules/sync", latestSync)
	e.GET("/services", listServices)
	e.POST("/resources", serviceCreate)
	e.GET("/resources/plans", servicePlans)
	e.GET("/resources/:instance", serviceInfo)
	e.DELETE("/resources/:instance", serviceDelete)
	e.GET("/resources/:instance/status", serviceStatus)
	e.POST("/resources/:instance/bind-app", serviceBind)
	e.DELETE("/resources/:instance/bind-app", serviceUnbind)
	e.POST("/resources/:instance/bind", serviceBindUnit)
	e.DELETE("/resources/:instance/bind", serviceUnbindUnit)
	e.GET("/resources/:instance/rule", serviceListRules)
	e.POST("/resources/:instance/rule", serviceAddRule)
	e.POST("/resources/:instance/sync", serviceForceSyncRule)
	e.DELETE("/resources/:instance/rule/:rule", serviceRemoveRule)

	e.GET("/apps/:app/rules", appRules)
	e.POST("/apps/:app/sync", appForceSyncRule)
}

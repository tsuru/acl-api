// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tsuru/acl-api/api"
	"github.com/tsuru/acl-api/api/version"
)

var (
	cfgFile string
)

var rootRun = func(cmd *cobra.Command, args []string) error {
	return api.StartAPI()
}

func makeCmds() *cobra.Command {
	var rootCmd = &cobra.Command{
		Version: version.Version,
		Use:     "acl-api",
		Short:   "Manage Tsuru App ACLs",
		RunE:    rootRun,
	}

	var apiCmd = &cobra.Command{
		Use:   "api",
		Short: "Run acl-api API and worker",
		RunE:  rootRun,
	}

	var workerCmd = &cobra.Command{
		Use:   "worker",
		Short: "Run acl-api worker only",
		RunE: func(cmd *cobra.Command, args []string) error {
			return api.StartWorker()
		},
	}

	rootCmd.AddCommand(apiCmd)
	rootCmd.AddCommand(workerCmd)

	return rootCmd
}

func Execute() error {
	rootCmd := makeCmds()
	err := initRootCmd(rootCmd)
	if err != nil {
		return err
	}
	return rootCmd.Execute()
}

func initRootCmd(rootCmd *cobra.Command) error {
	flags := rootCmd.PersistentFlags()
	flags.StringVar(&cfgFile, "config", "", "config file (default is $HOME/.acl.yaml)")
	flags.Parse(os.Args[1:])

	flags.Bool("debug", false, "Debug mode")
	flags.String("loglevel", "info", "Logrus log level")
	flags.String("storage", "", "Storage address")
	flags.StringSlice("engines", []string{"acl-operator"}, "Enabled syncing engines")
	flags.String("tsuru.host", "", "Tsuru URL")
	flags.String("tsuru.token", "", "Tsuru Token")

	flags.String("auth.user", "", "Auth User")
	flags.String("auth.password", "", "Auth Password")
	flags.String("auth.read_only_user", "", "Auth Read only User")
	flags.String("auth.read_only_password", "", "Auth Read only Password")

	flags.String("kubernetes.namespace", "tsuru", "Default Kubernetes namespace for tsuru")

	flags.Bool("tls.insecure", false, "Trust Any TLS Certificate")
	flags.Int("port", 8888, "Port to listen")
	flags.Duration("sync.interval", time.Minute, "Rules sync interval")
	flags.Duration("http.timeout", time.Minute, "Default HTTP timeout")

	initConfig(rootCmd)
	initLogging()

	return nil
}

func initConfig(rootCmd *cobra.Command) {
	// Log level will change after initLogging is called
	logrus.SetLevel(logrus.DebugLevel)

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName(".acl")
		viper.AddConfigPath("$HOME")
	}

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv()

	if err := viper.BindPFlags(rootCmd.PersistentFlags()); err != nil {
		logrus.Fatalf("unable to bind flags to viper: %v\n", err)
	}

	if err := viper.ReadInConfig(); err == nil {
		logrus.Info("Using config file:", viper.ConfigFileUsed())
	}
}

func initLogging() {
	if viper.GetBool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
		return
	}
	level := logrus.InfoLevel
	configLevel := viper.GetString("loglevel")
	if configLevel != "" {
		var err error
		level, err = logrus.ParseLevel(configLevel)
		if err != nil {
			logrus.Fatalf("invalid loglevel: %v", err)
		}
	}
	logrus.SetLevel(level)
}

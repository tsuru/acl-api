// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecute_flagParsing(t *testing.T) {
	tests := []struct {
		args          []string
		envs          map[string]string
		expected      map[string]interface{}
		expectedBool  map[string]bool
		expectedSlice map[string][]string
		config        string
	}{
		{
			args: []string{"--tsuru.host", "http://mytsuru1.com"},
			expected: map[string]interface{}{
				"tsuru.host": "http://mytsuru1.com",
			},
		},

		{
			config: `{"tsuru": {"host": "http://mytsuru2.com"}}`,
			expected: map[string]interface{}{
				"tsuru.host": "http://mytsuru2.com",
			},
		},

		{
			args:   []string{"--tsuru.host", "http://mytsuru9.com"},
			config: `{"tsuru": {"host": "http://mytsuru3.com"}}`,
			expected: map[string]interface{}{
				"tsuru.host": "http://mytsuru9.com",
			},
		},

		{
			envs:   map[string]string{"TSURU_HOST": "http://mytsuru20.com"},
			config: `{"tsuru": {"host": "http://mytsuru3.com"}}`,
			expected: map[string]interface{}{
				"tsuru.host": "http://mytsuru20.com",
			},
		},
	}

	rootRun = func(cmd *cobra.Command, args []string) error {
		return nil
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			viper.Reset()
			os.Args = append([]string{"acl-api"}, tt.args...)
			for k, v := range tt.envs {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}
			if tt.config != "" {
				name, err := ioutil.TempDir("", "")
				require.NoError(t, err)
				defer os.RemoveAll(name)
				name = filepath.Join(name, "config.json")
				err = ioutil.WriteFile(name, []byte(tt.config), 0400)
				require.NoError(t, err)
				os.Args = append(os.Args, "--config", name)
			}
			pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
			err := Execute()
			assert.NoError(t, err)
			for k, v := range tt.expected {
				val := viper.Get(k)
				assert.Equal(t, v, val)
			}
			for k, v := range tt.expectedBool {
				val := viper.GetBool(k)
				assert.Equal(t, v, val)
			}
			for k, v := range tt.expectedSlice {
				val := viper.GetStringSlice(k)
				assert.Equal(t, v, val)
			}
		})
	}
}

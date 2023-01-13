// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mongodb

import (
	"sync"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/tsuru/acl-api/storage"
)

func init() {
	viper.AutomaticEnv()
}

func TestGetRuleStorageInvalidURL(t *testing.T) {
	once = sync.Once{}
	defer viper.Set("storage", viper.Get("storage"))
	viper.Set("storage", "@@@")
	_, err := storage.GetRuleStorage()
	assert.NotNil(t, err)
}

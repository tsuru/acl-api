// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mongodb

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tsuru/acl-api/storage"
	"github.com/tsuru/acl-api/storage/storagetest"
)

func init() {
	viper.AutomaticEnv()
}

func TestACLAPIStorageSuite(t *testing.T) {
	defer viper.Set("storage", viper.Get("storage"))
	storagePath := viper.GetString("storage")
	if storagePath == "" {
		storagePath = "mongodb://localhost"
	}
	viper.Set("storage", storagePath+"/acltest-pkg-storage")
	stor, err := storage.GetACLAPIStorage()
	require.Nil(t, err)
	suite.Run(t, &storagetest.ACLAPIStorageSuite{
		Stor: stor,
		SetupTestFunc: func() {
			stor.(interface {
				ClearAll()
			}).ClearAll()
		},
	})
}

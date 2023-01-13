// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rule

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/acl-api/api/types"
	_ "github.com/tsuru/acl-api/storage/mongodb"
)

func Test_LogicCache_LogicFromRuleType(t *testing.T) {
	c := NewLogicCache()
	cachedLogic1, err := c.LogicFromRule(types.Rule{
		Source: types.RuleType{
			TsuruApp: &types.TsuruAppRule{
				AppName: "app1",
			},
		},
	})
	require.Nil(t, err)
	assert.NotNil(t, cachedLogic1)
	cachedLogic2, err := c.LogicFromRule(types.Rule{
		Source: types.RuleType{
			TsuruApp: &types.TsuruAppRule{
				AppName: "app1",
			},
		},
	})
	require.Nil(t, err)
	ptr1 := fmt.Sprintf("%x\n", cachedLogic1)
	ptr2 := fmt.Sprintf("%x\n", cachedLogic2)
	assert.Equal(t, ptr1, ptr2)
}

// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rule

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tsuru/acl-api/api/types"
	"github.com/tsuru/acl-api/external"
)

func Test_tsuruAppRuleLogic_FriendlyName(t *testing.T) {
	tests := []struct {
		rule *types.TsuruAppRule
		want string
	}{
		{rule: &types.TsuruAppRule{}, want: ""},
		{rule: &types.TsuruAppRule{AppName: "myapp"}, want: "myapp"},
	}
	for _, tt := range tests {
		s := &tsuruAppRuleLogic{
			rule:        tt.rule,
			tsuruClient: external.NewTsuruClient(),
		}
		assert.Equal(t, s.FriendlyName(), tt.want)
	}
}

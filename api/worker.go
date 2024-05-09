// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"github.com/google/gops/agent"
)

func StartWorker() error {
	if err := agent.Listen(agent.Options{}); err != nil {
		return err
	}
	defer agent.Close()

	setupEngine()
	return nil
}

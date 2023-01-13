// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

type ServiceRule struct {
	Rule
	Creator string
	EventID string
}

func (s *ServiceRule) Equals(other *ServiceRule) bool {
	return s.Rule.Destination.Equals(&other.Rule.Destination)
}

type ServiceInstance struct {
	InstanceName string
	Creator      string
	EventID      string
	BindApps     []string
	BaseRules    []ServiceRule
}

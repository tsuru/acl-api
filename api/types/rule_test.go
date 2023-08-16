// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRuleType(t *testing.T) {
	invalidDNSMsg := "DNS Rule: Name must be a valid DNS name, a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')"
	tests := []struct {
		rt       RuleType
		expected string
	}{
		{
			rt:       RuleType{},
			expected: "exactly one rule type must be set",
		},
		{
			rt: RuleType{
				ExternalDNS: &ExternalDNSRule{
					Name: "123InvalidDomain",
				},
			},
			expected: invalidDNSMsg,
		},
		{
			rt: RuleType{
				ExternalDNS: &ExternalDNSRule{
					Name: ".googleapis.com",
				},
			},
		},

		{
			rt: RuleType{
				ExternalDNS: &ExternalDNSRule{
					Name: "myservice.mynamespace.svc.cluster.local",
				},
			},
			expected: `DNS Rule: Name must not be a cluster internal address`,
		},

		{
			rt: RuleType{
				ExternalDNS: &ExternalDNSRule{
					Name: "https://myservice.globo.com",
				},
			},
			expected: invalidDNSMsg,
		},

		{
			rt: RuleType{
				ExternalDNS: &ExternalDNSRule{
					Name: "http://myservice.globo.com",
				},
			},
			expected: invalidDNSMsg,
		},

		{
			rt: RuleType{
				ExternalDNS: &ExternalDNSRule{
					Name: "myservice.globo.com:8080",
				},
			},
			expected: invalidDNSMsg,
		},

		{
			rt: RuleType{
				ExternalIP: &ExternalIPRule{
					IP: "10.1.1.1",
				},
			},
		},
		{
			rt: RuleType{
				ExternalIP: &ExternalIPRule{
					IP: "10.1.1.1/8",
				},
			},
			expected: `IP Rule: Large CIDR, the maximum size of network without ports is /22`,
		},
		{
			rt: RuleType{
				ExternalIP: &ExternalIPRule{
					IP: "10.1.1.1/8",
					Ports: []ProtoPort{
						{Protocol: "TCP", Port: 80},
					},
				},
			},
		},
		{
			rt: RuleType{
				ExternalIP: &ExternalIPRule{
					IP: "2001:db8:a0b:12f0::1/32",
					Ports: []ProtoPort{
						{Protocol: "TCP", Port: 80},
					},
				},
			},
			expected: `IP Rule: Invalid IP, IPv6 is not supported yet`,
		},
		{
			rt: RuleType{
				RpaasInstance: &RpaasInstanceRule{},
			},
			expected: `cannot have empty rpaas serviceName or instance`,
		},
		{
			rt: RuleType{
				RpaasInstance: &RpaasInstanceRule{
					ServiceName: "blaah-blah/blah",
					Instance:    "blah",
				},
			},
			expected: `Invalid rpaas service name`,
		},
		{
			rt: RuleType{
				RpaasInstance: &RpaasInstanceRule{
					ServiceName: "blaah-blah",
					Instance:    "blah-blah123/k",
				},
			},
			expected: `Invalid rpaas instance name`,
		},
		{
			rt: RuleType{
				TsuruApp: &TsuruAppRule{
					AppName:  "blaah-blah",
					PoolName: "pool-name",
				},
			},
			expected: `cannot set both app name and pool name`,
		},
		{
			rt: RuleType{
				TsuruApp: &TsuruAppRule{
					AppName: "blaah-blah.cluster",
				},
			},
			expected: `invalid app name`,
		},
		{
			rt: RuleType{
				TsuruJob: &TsuruJobRule{
					JobName: "blaah-blah.cluster",
				},
			},
			expected: `invalid job name`,
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			err := tt.rt.Validate()
			if tt.expected == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Equal(t, tt.expected, err.Error())
			}
		})
	}
}

func TestEqualRuleType(t *testing.T) {
	tests := []struct {
		rt1      RuleType
		rt2      RuleType
		expected bool
	}{
		{
			rt1:      RuleType{},
			rt2:      RuleType{},
			expected: true,
		},

		{
			rt1: RuleType{
				TsuruApp: &TsuruAppRule{
					AppName: "app1",
				},
			},
			rt2: RuleType{
				TsuruApp: &TsuruAppRule{
					AppName: "app2",
				},
			},
			expected: false,
		},

		{
			rt1: RuleType{
				ExternalDNS: &ExternalDNSRule{
					Name: "myservice.mynamespace.svc.cluster.local",
				},
			},
			rt2: RuleType{
				ExternalDNS: &ExternalDNSRule{
					Name: "other.google.com",
				},
			},
			expected: false,
		},

		{
			rt1: RuleType{
				ExternalDNS: &ExternalDNSRule{
					Name: "myservice.mynamespace.svc.cluster.local",
				},
			},
			rt2:      RuleType{},
			expected: false,
		},

		{
			rt1: RuleType{
				ExternalDNS: &ExternalDNSRule{
					Name: "app1.globoi.com",
					Ports: ProtoPorts{
						{Protocol: "TCP", Port: 443},
						{Protocol: "TCP", Port: 80},
					},
				},
			},
			rt2: RuleType{
				ExternalDNS: &ExternalDNSRule{
					Name: "app1.globoi.com",
					Ports: ProtoPorts{
						{Protocol: "tcp", Port: 80},
						{Protocol: "tcp", Port: 443},
					},
				},
			},
			expected: true,
		},

		{
			rt1: RuleType{
				ExternalDNS: &ExternalDNSRule{
					Name: "app1.globoi.com",
				},
			},
			rt2: RuleType{
				ExternalDNS: &ExternalDNSRule{
					Name: "app2.globoi.com",
				},
			},
			expected: false,
		},

		{
			rt1: RuleType{
				ExternalDNS: &ExternalDNSRule{
					Name: "app1.globoi.com",
					Ports: ProtoPorts{
						{Protocol: "TCP", Port: 443},
						{Protocol: "TCP", Port: 8888},
					},
				},
			},
			rt2: RuleType{
				ExternalDNS: &ExternalDNSRule{
					Name: "app1.globoi.com",
					Ports: ProtoPorts{
						{Protocol: "tcp", Port: 80},
						{Protocol: "tcp", Port: 443},
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.rt1.Equals(&tt.rt2))
		})
	}
}

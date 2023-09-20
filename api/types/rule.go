// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/validation"
)

var tsuruNameRegexp = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)

type Rule struct {
	RuleID      string
	RuleName    string
	Source      RuleType
	Destination RuleType
	Removed     bool
	Metadata    map[string]string
	Created     time.Time
	Creator     string
}

type RuleSyncInfo struct {
	SyncID    string
	RuleID    string
	Engine    string
	StartTime time.Time
	EndTime   time.Time
	PingTime  time.Time
	Running   bool
	Syncs     []RuleSyncData
}

func (rsi RuleSyncInfo) LatestSync() *RuleSyncData {
	if len(rsi.Syncs) == 0 {
		return nil
	}
	return &rsi.Syncs[len(rsi.Syncs)-1]
}

type RuleSyncData struct {
	StartTime  time.Time
	EndTime    time.Time
	Successful bool
	Removed    bool
	Error      string
	SyncResult string
}

type RuleType struct {
	TsuruApp          *TsuruAppRule          `json:"TsuruApp,omitempty"`
	TsuruJob          *TsuruJobRule          `json:"TsuruJob,omitempty"`
	KubernetesService *KubernetesServiceRule `json:"KubernetesService,omitempty"`
	ExternalDNS       *ExternalDNSRule       `json:"ExternalDNS,omitempty"`
	ExternalIP        *ExternalIPRule        `json:"ExternalIP,omitempty"`
	RpaasInstance     *RpaasInstanceRule     `json:"RpaasInstance,omitempty"`
}

func (r *RuleType) Validate() error {
	countSet := 0
	if r.TsuruApp != nil {
		if r.TsuruApp.AppName == "" && r.TsuruApp.PoolName == "" {
			return errors.New("cannot have empty tsuru app name and pool name")
		}
		if r.TsuruApp.AppName != "" && r.TsuruApp.PoolName != "" {
			return errors.New("cannot set both app name and pool name")
		}

		if r.TsuruApp.AppName != "" && !validateTsuruName(r.TsuruApp.AppName) {
			return errors.New("invalid app name")
		}
		countSet++
	}

	if r.TsuruJob != nil {
		if r.TsuruJob.JobName == "" {
			return errors.New("cannot have empty tsuru job name")
		}

		if !validateTsuruName(r.TsuruJob.JobName) {
			return errors.New("invalid job name")
		}
		countSet++
	}

	if r.RpaasInstance != nil {
		if r.RpaasInstance.ServiceName == "" || r.RpaasInstance.Instance == "" {
			return errors.New("cannot have empty rpaas serviceName or instance")
		}
		if errs := validation.IsDNS1035Label(r.RpaasInstance.ServiceName); len(errs) > 0 {
			return errors.New("Invalid rpaas service name")
		}

		if errs := validation.IsDNS1035Label(r.RpaasInstance.Instance); len(errs) > 0 {
			return errors.New("Invalid rpaas instance name")
		}

		countSet++
	}

	var ports []ProtoPort
	if r.ExternalDNS != nil {
		if r.ExternalDNS.Name == "" {
			return errors.New("cannot have empty external dns name")
		}
		ports = r.ExternalDNS.Ports
		nameToValidate := r.ExternalDNS.Name
		if nameToValidate[0] == '.' {
			nameToValidate = nameToValidate[1:]
		}

		if errs := validation.IsDNS1123Subdomain(nameToValidate); len(errs) > 0 {
			return errors.New("DNS Rule: Name must be a valid DNS name, " + strings.Join(errs, ", "))
		}

		if strings.HasSuffix(r.ExternalDNS.Name, "cluster.local") {
			return errors.New("DNS Rule: Name must not be a cluster internal address")
		}
		countSet++
	}

	if r.ExternalIP != nil {
		ports = r.ExternalIP.Ports
		if r.ExternalIP.IP == "" {
			return errors.New("cannot have empty external ip address")
		}
		ipToValidate := r.ExternalIP.IP
		if !strings.Contains(ipToValidate, "/") {
			if strings.Contains(ipToValidate, ":") {
				ipToValidate += "/128"
			} else if strings.Contains(ipToValidate, ".") {
				ipToValidate += "/32"
			}
		}
		_, ipNet, err := net.ParseCIDR(ipToValidate)
		if err != nil {
			return errors.New("IP Rule: Invalid IP, " + err.Error())
		}

		ones, bits := ipNet.Mask.Size()
		if bits == 128 {
			return errors.New("IP Rule: Invalid IP, IPv6 is not supported yet")
		}

		if ones < 22 && bits == 32 && len(r.ExternalIP.Ports) == 0 {
			return errors.New("IP Rule: Large CIDR, the maximum size of network without ports is /22")
		}
		countSet++
	}

	if r.KubernetesService != nil {
		// desativamos devido ao uso incorreto
		// thread: https://globo.slack.com/archives/G62GPMXKN/p1637761216109500
		return errors.New("Kubernetes Service Rule: has been deactivated for use, please use instead: App or RPaaS destinations")
	}

	if countSet != 1 {
		return errors.New("exactly one rule type must be set")
	}

	err := validatePorts(ports)
	if err != nil {
		return err
	}

	return nil
}

func (r *RuleType) Equals(other *RuleType) bool {
	if r == nil && other == nil {
		return true
	}
	if r == nil || other == nil {
		return true
	}

	if r.TsuruApp != nil {
		if !reflect.DeepEqual(r.TsuruApp, other.TsuruApp) {
			return false
		}
	}

	if r.KubernetesService != nil {
		if !reflect.DeepEqual(r.KubernetesService, other.KubernetesService) {
			return false
		}
	}

	if r.ExternalDNS != nil {
		if !r.ExternalDNS.Equals(other.ExternalDNS) {
			return false
		}
	}

	if r.ExternalIP != nil {
		if !r.ExternalIP.Equals(other.ExternalIP) {
			return false
		}
	}

	if r.RpaasInstance != nil {
		if !reflect.DeepEqual(r.RpaasInstance, other.RpaasInstance) {
			return false
		}
	}

	return true
}

func validatePorts(ports []ProtoPort) error {
	validProtos := map[string]struct{}{"TCP": {}, "UDP": {}}

	for _, p := range ports {
		if p.Port == 0 {
			return errors.Errorf("invalid port number 0")
		}
		if _, isValid := validProtos[strings.ToUpper(p.Protocol)]; isValid {
			continue
		}
		validProtoStrs := make([]string, 0, len(validProtos))
		for proto := range validProtos {
			validProtoStrs = append(validProtoStrs, proto)
		}
		sort.Strings(validProtoStrs)
		return errors.Errorf("invalid protocol %q, valid values are: %v", p.Protocol, strings.Join(validProtoStrs, ", "))
	}
	return nil
}

func validateTsuruName(name string) bool {
	return tsuruNameRegexp.MatchString(name)
}

type ProtoPorts []ProtoPort

type ProtoPort struct {
	Protocol string
	Port     uint16
}

func (p ProtoPort) String() string {
	return fmt.Sprintf("%s:%d", p.Protocol, p.Port)
}

type TsuruAppRule struct {
	AppName  string
	PoolName string
}

type TsuruJobRule struct {
	JobName string
}

type KubernetesServiceRule struct {
	Namespace   string
	ServiceName string
	ClusterName string
}

type ExternalDNSRule struct {
	Name             string
	Ports            ProtoPorts
	SyncWholeNetwork bool
}

type ExternalIPRule struct {
	IP               string
	Ports            ProtoPorts
	SyncWholeNetwork bool
}

type RpaasInstanceRule struct {
	ServiceName string
	Instance    string
}

func (r *RpaasInstanceRule) String() string {
	return fmt.Sprintf("Rpaas: %s/%s", r.ServiceName, r.Instance)
}

func prettyPorts(ports []ProtoPort) string {
	if len(ports) == 0 {
		return ""
	}
	strs := make([]string, len(ports))
	for i, p := range ports {
		strs[i] = fmt.Sprintf("%s:%d", p.Protocol, p.Port)
	}
	sort.Strings(strs)
	return fmt.Sprintf(", Ports: %s", strings.Join(strs, ", "))
}

func (rt *RuleType) String() string {
	if rt.TsuruApp != nil {
		if rt.TsuruApp.AppName == "" && rt.TsuruApp.PoolName != "" {
			return fmt.Sprintf("Tsuru Pool: %s", rt.TsuruApp.PoolName)
		}
		return fmt.Sprintf("Tsuru APP: %s", rt.TsuruApp.AppName)
	}
	if rt.TsuruJob != nil {
		return fmt.Sprintf("Tsuru Job: %s", rt.TsuruJob.JobName)
	}
	if rt.ExternalDNS != nil {
		wholeNet := ""
		if rt.ExternalDNS.SyncWholeNetwork {
			wholeNet = ", whole network"
		}
		return fmt.Sprintf("DNS: %s%s%s", rt.ExternalDNS.Name, prettyPorts(rt.ExternalDNS.Ports), wholeNet)
	}
	if rt.ExternalIP != nil {
		wholeNet := ""
		if rt.ExternalIP.SyncWholeNetwork {
			wholeNet = ", whole network"
		}
		return fmt.Sprintf("IP: %s%s%s", rt.ExternalIP.IP, prettyPorts(rt.ExternalIP.Ports), wholeNet)
	}
	if rt.KubernetesService != nil {
		if rt.KubernetesService.Namespace == "" {
			rt.KubernetesService.Namespace = "default"
		}
		return fmt.Sprintf("Kubernetes Service: %s/%s", rt.KubernetesService.Namespace, rt.KubernetesService.ServiceName)
	}
	if rt.RpaasInstance != nil {
		return rt.RpaasInstance.String()
	}

	return ""
}

func (rt *RuleType) CacheKey() (string, error) {
	result, err := json.Marshal(rt)
	if err != nil {
		return "", errors.Wrap(err, "could not create cache key for rule type")
	}
	return string(result), nil
}

func (r *Rule) String() string {
	return fmt.Sprintf("[RuleID: %v, Source: %s, Destination: %s]",
		r.RuleID,
		r.Source.String(),
		r.Destination.String(),
	)
}

func (t *ExternalDNSRule) Equals(other *ExternalDNSRule) bool {
	if other == nil {
		return false
	}
	if t.Name != other.Name {
		return false
	}

	if t.SyncWholeNetwork != other.SyncWholeNetwork {
		return false
	}

	if t.Ports != nil {
		return t.Ports.Equals(other.Ports)
	}

	return true
}

func (t *ExternalIPRule) Equals(other *ExternalIPRule) bool {
	if other == nil {
		return false
	}
	if t.IP != other.IP {
		return false
	}

	if t.SyncWholeNetwork != other.SyncWholeNetwork {
		return false
	}

	if t.Ports != nil {
		return t.Ports.Equals(other.Ports)
	}

	return true
}

func (p ProtoPorts) Equals(other ProtoPorts) bool {
	if len(p) != len(other) {
		return false
	}

	originTCPPorts := make(map[uint16]struct{})
	originUDPPorts := make(map[uint16]struct{})

	otherTCPPorts := make(map[uint16]struct{})
	otherUDPPorts := make(map[uint16]struct{})

	for _, port := range p {
		if strings.ToLower(port.Protocol) == "tcp" {
			originTCPPorts[port.Port] = struct{}{}
		}

		if strings.ToLower(port.Protocol) == "udp" {
			originUDPPorts[port.Port] = struct{}{}
		}
	}

	for _, port := range other {
		if strings.ToLower(port.Protocol) == "tcp" {
			otherTCPPorts[port.Port] = struct{}{}
		}

		if strings.ToLower(port.Protocol) == "udp" {
			otherUDPPorts[port.Port] = struct{}{}
		}
	}

	if !reflect.DeepEqual(originTCPPorts, otherTCPPorts) {
		return false
	}

	if !reflect.DeepEqual(originUDPPorts, otherUDPPorts) {
		return false
	}

	return true
}

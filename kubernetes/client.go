// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kubernetes

import (
	"errors"
	"math/rand"

	"github.com/spf13/viper"
	"github.com/tsuru/acl-api/external"
	tsuruv1clientset "github.com/tsuru/tsuru/provision/kubernetes/pkg/client/clientset/versioned"
	provTypes "github.com/tsuru/tsuru/types/provision"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

const (
	tokenClusterKey    = "token"
	userClusterKey     = "username"
	passwordClusterKey = "password"
)

// GetClient func may be overridden in tests
var GetClient = func(cluster provTypes.Cluster) (kubernetes.Interface, error) {
	config, err := RestConfig(cluster)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

var GetClientWithRestConfig = func(config *rest.Config) (kubernetes.Interface, error) {
	return kubernetes.NewForConfig(config)
}

// GetClientset func may be overridden in tests
var GetClientset = func(cluster provTypes.Cluster) (apiextensionsclientset.Interface, error) {
	config, err := RestConfig(cluster)
	if err != nil {
		return nil, err
	}
	return apiextensionsclientset.NewForConfig(config)
}

// GetTsuruClient func may be overridden in tests
var GetTsuruClient = func(cluster provTypes.Cluster) (tsuruv1clientset.Interface, error) {
	config, err := RestConfig(cluster)
	if err != nil {
		return nil, err
	}
	return tsuruv1clientset.NewForConfig(config)
}

var GetTsuruClientWithRestConfig = func(config *rest.Config) (tsuruv1clientset.Interface, error) {
	return tsuruv1clientset.NewForConfig(config)
}

var RestConfig = func(cluster provTypes.Cluster) (*rest.Config, error) {
	gv, err := schema.ParseGroupVersion("/v1")
	if err != nil {
		return nil, err
	}

	if cluster.KubeConfig != nil {
		return getRestConfigByKubeConfig(cluster)
	}

	cfg := &rest.Config{
		APIPath: "/api",
		ContentConfig: rest.ContentConfig{
			GroupVersion:         &gv,
			NegotiatedSerializer: serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs},
		},
		Timeout: viper.GetDuration("http.timeout"),
	}
	if len(cluster.Addresses) == 0 {
		return nil, errors.New("no addresses for cluster")
	}
	addr := cluster.Addresses[rand.Intn(len(cluster.Addresses))]
	var token, user, password string
	if cluster.CustomData != nil {
		token = cluster.CustomData[tokenClusterKey]
		user = cluster.CustomData[userClusterKey]
		password = cluster.CustomData[passwordClusterKey]
	}
	cfg.Host = addr
	cfg.TLSClientConfig = rest.TLSClientConfig{
		CAData:   cluster.CaCert,
		CertData: cluster.ClientCert,
		KeyData:  cluster.ClientKey,
	}
	if user != "" && password != "" {
		cfg.Username = user
		cfg.Password = password
	} else {
		cfg.BearerToken = token
	}
	cfg.Wrap(external.MetricsRoundTripper)
	return cfg, nil
}

func getRestConfigByKubeConfig(cluster provTypes.Cluster) (*rest.Config, error) {
	gv, err := schema.ParseGroupVersion("/v1")
	if err != nil {
		return nil, err
	}

	cliCfg := clientcmdapi.Config{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: cluster.Name,
		Clusters: map[string]*clientcmdapi.Cluster{
			cluster.Name: &cluster.KubeConfig.Cluster,
		},
		Contexts: map[string]*clientcmdapi.Context{
			cluster.Name: {
				Cluster:  cluster.Name,
				AuthInfo: cluster.Name,
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			cluster.Name: &cluster.KubeConfig.AuthInfo,
		},
	}
	restConfig, err := clientcmd.NewNonInteractiveClientConfig(cliCfg, cluster.Name, &clientcmd.ConfigOverrides{}, nil).ClientConfig()
	if err != nil {
		return nil, err
	}

	restConfig.APIPath = "/api"
	restConfig.ContentConfig = rest.ContentConfig{
		GroupVersion:         &gv,
		NegotiatedSerializer: serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs},
	}
	restConfig.Timeout = viper.GetDuration("http.timeout")

	return restConfig, nil
}

func DefaultNamespace() string {
	return viper.GetString("kubernetes.namespace")
}

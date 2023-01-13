// Copyright 2023 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package external

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	userAgent = "acl-api-http-client/1.0"

	promNamespace = "acl_api"
	promSubsystem = "external"
)

var (
	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: promNamespace,
		Subsystem: promSubsystem,
		Name:      "request_duration_seconds",
		Help:      "External http request duration seconds",
		Buckets:   prometheus.ExponentialBuckets(0.1, 2.1, 10),
	}, []string{"host", "method", "code"})
)

var (
	baseClient *http.Client
	onceClient sync.Once
)

type BaseHTTPClient struct {
	sync.RWMutex
	client      *http.Client
	Timeout     time.Duration
	URL         string
	User        string
	Password    string
	Token       string
	OAuthURL    string
	OAuthId     string
	OAuthSecret string
	Logger      logrus.FieldLogger
}

func createBaseClient() *http.Client {
	onceClient.Do(func() {
		baseClient = &http.Client{
			Transport: MetricsRoundTripper(&http.Transport{
				Dial: (&net.Dialer{
					Timeout:   20 * time.Second,
					KeepAlive: 15 * time.Second,
				}).Dial,
				TLSHandshakeTimeout: 10 * time.Second,
				IdleConnTimeout:     20 * time.Second,
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: viper.GetBool("tls.insecure"),
				},
			}),
			Timeout: viper.GetDuration("http.timeout"),
		}
	})
	return baseClient
}

func (e *BaseHTTPClient) GetClient() (*http.Client, error) {
	e.RLock()
	if e.client != nil {
		defer e.RUnlock()
		return e.client, nil
	}
	e.RUnlock()
	e.Lock()
	defer e.Unlock()
	if e.client != nil {
		return e.client, nil
	}
	e.client = createBaseClient()
	if e.Timeout > 0 {
		cli := *e.client
		e.client = &cli
		e.client.Timeout = e.Timeout
	}
	var oauthConfig *clientcredentials.Config
	if e.OAuthURL != "" {
		oauthConfig = &clientcredentials.Config{
			ClientID:     e.OAuthId,
			ClientSecret: e.OAuthSecret,
			TokenURL:     e.OAuthURL,
		}
	}
	if oauthConfig != nil {
		ctx := context.WithValue(context.Background(), oauth2.HTTPClient, e.client)
		e.client = oauthConfig.Client(ctx)
	}
	return e.client, nil
}

func MetricsRoundTripper(next http.RoundTripper) http.RoundTripper {
	return promhttp.RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		start := time.Now()
		resp, err := next.RoundTrip(r)
		code := "error"
		if err == nil {
			code = strconv.Itoa(resp.StatusCode)
		}
		httpRequestDuration.WithLabelValues(r.URL.Host, r.Method, code).Observe(time.Since(start).Seconds())
		return resp, err
	})
}

func isDebug() bool {
	return logrus.GetLevel() >= logrus.DebugLevel
}

func (e *BaseHTTPClient) DoRequestDataCtx(ctx context.Context, method, path string, body io.Reader, headers map[string]string) ([]byte, error) {
	rsp, err := e.doRequest(ctx, method, path, body, headers)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()
	data, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to read response body")
	}
	return data, nil
}

func (e *BaseHTTPClient) DoRequestData(method, path string, body io.Reader, headers map[string]string) ([]byte, error) {
	return e.DoRequestDataCtx(context.TODO(), method, path, body, headers)
}

type HTTPError struct {
	StatusCode int
	Body       string
	Headers    http.Header
}

func (e *HTTPError) Error() string {
	reqID := e.Headers.Get("X-Request-ID")
	if reqID == "" {
		reqID = e.Headers.Get("X-RID")
	}
	if reqID == "" {
		reqID = e.Headers.Get("X-Requestid")
	}

	var reqIDMsg string
	if reqID != "" {
		reqIDMsg = fmt.Sprintf("(request-id: %s)", reqID)
	}
	return fmt.Sprintf("invalid status code %d%s: %q", e.StatusCode, reqIDMsg, e.Body)
}

func (e *BaseHTTPClient) doRequest(ctx context.Context, method, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	fullUrl := fmt.Sprintf("%s/%s", strings.TrimSuffix(e.URL, "/"), strings.TrimPrefix(path, "/"))
	bodyBuf := bytes.Buffer{}
	if body != nil && isDebug() {
		body = io.TeeReader(body, &bodyBuf)
	}
	req, err := http.NewRequest(method, fullUrl, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	cli, err := e.GetClient()
	if err != nil {
		return nil, err
	}
	if e.User != "" {
		req.SetBasicAuth(e.User, e.Password)
	}
	if e.Token != "" {
		req.Header.Set("Authorization", "Bearer "+e.Token)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	for h, v := range headers {
		req.Header.Set(h, v)
	}
	t0 := time.Now()
	rsp, err := cli.Do(req)
	reqDuration := time.Since(t0)
	statusCodeValid := true
	if err == nil {
		statusCodeValid = rsp.StatusCode >= 200 && rsp.StatusCode < 400
	}
	var rspData []byte
	if !statusCodeValid {
		rspData, _ = io.ReadAll(rsp.Body)
		rsp.Body.Close()
	}
	if isDebug() {
		var debugRsp string
		if err != nil {
			debugRsp = fmt.Sprintf("req-body: %s - err: %v", bodyBuf.String(), err.Error())
		} else {
			if statusCodeValid {
				debugRsp = fmt.Sprintf("status code: %v", rsp.StatusCode)
			} else {
				debugRsp = fmt.Sprintf("status code: %v - req-body: %s - rsp-body: %s - rsp-headers: %+v", rsp.StatusCode, bodyBuf.String(), string(rspData), rsp.Header)
			}
		}
		e.Logger.Debugf("client request %s %s (%v) - %s", method, fullUrl, reqDuration, debugRsp)
	}
	if err != nil {
		return nil, err
	}
	if !statusCodeValid {
		return nil, errors.WithStack(&HTTPError{
			StatusCode: rsp.StatusCode,
			Body:       string(rspData),
			Headers:    rsp.Header,
		})
	}
	return rsp, nil
}

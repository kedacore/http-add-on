/*
Copyright 2024 The KEDA Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"fmt"

	"github.com/kedacore/http-add-on/pkg/build"
)

var (
	interceptorImage = fmt.Sprintf("ghcr.io/kedacore/http-add-on-interceptor:%s", build.Version())
	scalerImage      = fmt.Sprintf("ghcr.io/kedacore/http-add-on-scaler:%s", build.Version())
)

func (c *HTTPInterceptorSepc) GetProxyPort() int32 {
	if c.Config == nil || c.Config.ProxyPort == nil {
		return 8080
	}
	return *c.Config.ProxyPort
}

func (c *HTTPInterceptorSepc) GetAdminPort() int32 {
	if c.Config == nil || c.Config.AdminPort == nil {
		return 9090
	}
	return *c.Config.AdminPort
}
func (c *HTTPInterceptorSepc) GetConnectTimeout() string {
	if c.Config == nil || c.Config.ConnectTimeout == nil {
		return "500ms"
	}
	return *c.Config.ConnectTimeout
}
func (c *HTTPInterceptorSepc) GetHeaderTimeout() string {
	if c.Config == nil || c.Config.HeaderTimeout == nil {
		return "500ms"
	}
	return *c.Config.HeaderTimeout
}
func (c *HTTPInterceptorSepc) GetWaitTimeout() string {
	if c.Config == nil || c.Config.WaitTimeout == nil {
		return "1500ms"
	}
	return *c.Config.WaitTimeout
}
func (c *HTTPInterceptorSepc) GetIdleConnTimeout() string {
	if c.Config == nil || c.Config.IdleConnTimeout == nil {
		return "90s"
	}
	return *c.Config.IdleConnTimeout
}
func (c *HTTPInterceptorSepc) GetTLSHandshakeTimeout() string {
	if c.Config == nil || c.Config.TLSHandshakeTimeout == nil {
		return "10s"
	}
	return *c.Config.TLSHandshakeTimeout
}
func (c *HTTPInterceptorSepc) GetExpectContinueTimeout() string {
	if c.Config == nil || c.Config.ExpectContinueTimeout == nil {
		return "1s"
	}
	return *c.Config.ExpectContinueTimeout
}
func (c *HTTPInterceptorSepc) GetForceHTTP2() bool {
	if c.Config == nil || c.Config.ForceHTTP2 == nil {
		return false
	}
	return *c.Config.ForceHTTP2
}
func (c *HTTPInterceptorSepc) GetKeepAlive() string {
	if c.Config == nil || c.Config.KeepAlive == nil {
		return "1s"
	}
	return *c.Config.KeepAlive
}
func (c *HTTPInterceptorSepc) GetMaxIdleConns() int {
	if c.Config == nil || c.Config.MaxIdleConns == nil {
		return 100
	}
	return *c.Config.MaxIdleConns
}
func (c *HTTPInterceptorSepc) GetPollingInterval() int {
	if c.Config == nil || c.Config.PollingInterval == nil {
		return 1000
	}
	return *c.Config.PollingInterval
}

func (c *HTTPInterceptorSepc) GetImage() string {
	if c.Image == nil {
		return interceptorImage
	}
	return *c.Image
}

func (c *HTTPScalerSepc) GetPort() int32 {
	if c.Config.Port == nil {
		return 9090
	}
	return *c.Config.Port
}

func (c *HTTPScalerSepc) GetImage() string {
	if c.Image == nil {
		return scalerImage
	}
	return *c.Image
}

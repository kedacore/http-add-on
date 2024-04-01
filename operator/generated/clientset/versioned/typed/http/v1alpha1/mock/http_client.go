// /*
// Copyright 2023 The KEDA Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */
//

// Code generated by MockGen. DO NOT EDIT.
// Source: operator/generated/clientset/versioned/typed/http/v1alpha1/http_client.go

// Package mock is a generated GoMock package.
package mock

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	v1alpha1 "github.com/kedacore/http-add-on/operator/generated/clientset/versioned/typed/http/v1alpha1"
	rest "k8s.io/client-go/rest"
)

// MockHttpV1alpha1Interface is a mock of HttpV1alpha1Interface interface.
type MockHttpV1alpha1Interface struct {
	ctrl     *gomock.Controller
	recorder *MockHttpV1alpha1InterfaceMockRecorder
}

// MockHttpV1alpha1InterfaceMockRecorder is the mock recorder for MockHttpV1alpha1Interface.
type MockHttpV1alpha1InterfaceMockRecorder struct {
	mock *MockHttpV1alpha1Interface
}

// NewMockHttpV1alpha1Interface creates a new mock instance.
func NewMockHttpV1alpha1Interface(ctrl *gomock.Controller) *MockHttpV1alpha1Interface {
	mock := &MockHttpV1alpha1Interface{ctrl: ctrl}
	mock.recorder = &MockHttpV1alpha1InterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockHttpV1alpha1Interface) EXPECT() *MockHttpV1alpha1InterfaceMockRecorder {
	return m.recorder
}

// HTTPScaledObjects mocks base method.
func (m *MockHttpV1alpha1Interface) HTTPScaledObjects(namespace string) v1alpha1.HTTPScaledObjectInterface {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HTTPScaledObjects", namespace)
	ret0, _ := ret[0].(v1alpha1.HTTPScaledObjectInterface)
	return ret0
}

// HTTPScaledObjects indicates an expected call of HTTPScaledObjects.
func (mr *MockHttpV1alpha1InterfaceMockRecorder) HTTPScaledObjects(namespace interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HTTPScaledObjects", reflect.TypeOf((*MockHttpV1alpha1Interface)(nil).HTTPScaledObjects), namespace)
}

// RESTClient mocks base method.
func (m *MockHttpV1alpha1Interface) RESTClient() rest.Interface {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RESTClient")
	ret0, _ := ret[0].(rest.Interface)
	return ret0
}

// RESTClient indicates an expected call of RESTClient.
func (mr *MockHttpV1alpha1InterfaceMockRecorder) RESTClient() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RESTClient", reflect.TypeOf((*MockHttpV1alpha1Interface)(nil).RESTClient))
}

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
// Source: operator/generated/listers/http/v1alpha1/httpscalingset.go
//
// Generated by this command:
//
//	mockgen -copyright_file=hack/boilerplate.go.txt -destination=operator/generated/listers/http/v1alpha1/mock/httpscalingset.go -package=mock -source=operator/generated/listers/http/v1alpha1/httpscalingset.go
//

// Package mock is a generated GoMock package.
package mock

import (
	reflect "reflect"

	v1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	v1alpha10 "github.com/kedacore/http-add-on/operator/generated/listers/http/v1alpha1"
	gomock "go.uber.org/mock/gomock"
	labels "k8s.io/apimachinery/pkg/labels"
)

// MockHTTPScalingSetLister is a mock of HTTPScalingSetLister interface.
type MockHTTPScalingSetLister struct {
	ctrl     *gomock.Controller
	recorder *MockHTTPScalingSetListerMockRecorder
}

// MockHTTPScalingSetListerMockRecorder is the mock recorder for MockHTTPScalingSetLister.
type MockHTTPScalingSetListerMockRecorder struct {
	mock *MockHTTPScalingSetLister
}

// NewMockHTTPScalingSetLister creates a new mock instance.
func NewMockHTTPScalingSetLister(ctrl *gomock.Controller) *MockHTTPScalingSetLister {
	mock := &MockHTTPScalingSetLister{ctrl: ctrl}
	mock.recorder = &MockHTTPScalingSetListerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockHTTPScalingSetLister) EXPECT() *MockHTTPScalingSetListerMockRecorder {
	return m.recorder
}

// HTTPScalingSets mocks base method.
func (m *MockHTTPScalingSetLister) HTTPScalingSets(namespace string) v1alpha10.HTTPScalingSetNamespaceLister {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HTTPScalingSets", namespace)
	ret0, _ := ret[0].(v1alpha10.HTTPScalingSetNamespaceLister)
	return ret0
}

// HTTPScalingSets indicates an expected call of HTTPScalingSets.
func (mr *MockHTTPScalingSetListerMockRecorder) HTTPScalingSets(namespace any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HTTPScalingSets", reflect.TypeOf((*MockHTTPScalingSetLister)(nil).HTTPScalingSets), namespace)
}

// List mocks base method.
func (m *MockHTTPScalingSetLister) List(selector labels.Selector) ([]*v1alpha1.HTTPScalingSet, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List", selector)
	ret0, _ := ret[0].([]*v1alpha1.HTTPScalingSet)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockHTTPScalingSetListerMockRecorder) List(selector any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockHTTPScalingSetLister)(nil).List), selector)
}

// MockHTTPScalingSetNamespaceLister is a mock of HTTPScalingSetNamespaceLister interface.
type MockHTTPScalingSetNamespaceLister struct {
	ctrl     *gomock.Controller
	recorder *MockHTTPScalingSetNamespaceListerMockRecorder
}

// MockHTTPScalingSetNamespaceListerMockRecorder is the mock recorder for MockHTTPScalingSetNamespaceLister.
type MockHTTPScalingSetNamespaceListerMockRecorder struct {
	mock *MockHTTPScalingSetNamespaceLister
}

// NewMockHTTPScalingSetNamespaceLister creates a new mock instance.
func NewMockHTTPScalingSetNamespaceLister(ctrl *gomock.Controller) *MockHTTPScalingSetNamespaceLister {
	mock := &MockHTTPScalingSetNamespaceLister{ctrl: ctrl}
	mock.recorder = &MockHTTPScalingSetNamespaceListerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockHTTPScalingSetNamespaceLister) EXPECT() *MockHTTPScalingSetNamespaceListerMockRecorder {
	return m.recorder
}

// Get mocks base method.
func (m *MockHTTPScalingSetNamespaceLister) Get(name string) (*v1alpha1.HTTPScalingSet, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", name)
	ret0, _ := ret[0].(*v1alpha1.HTTPScalingSet)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockHTTPScalingSetNamespaceListerMockRecorder) Get(name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockHTTPScalingSetNamespaceLister)(nil).Get), name)
}

// List mocks base method.
func (m *MockHTTPScalingSetNamespaceLister) List(selector labels.Selector) ([]*v1alpha1.HTTPScalingSet, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List", selector)
	ret0, _ := ret[0].([]*v1alpha1.HTTPScalingSet)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockHTTPScalingSetNamespaceListerMockRecorder) List(selector any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockHTTPScalingSetNamespaceLister)(nil).List), selector)
}

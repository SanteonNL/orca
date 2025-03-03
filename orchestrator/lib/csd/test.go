// Code generated by MockGen. DO NOT EDIT.
// Source: interface.go
//
// Generated by this command:
//
//	mockgen -destination=./test.go -package=csd -source=interface.go
//

// Package csd is a generated GoMock package.
package csd

import (
	context "context"
	reflect "reflect"

	fhir "github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	gomock "go.uber.org/mock/gomock"
)

// MockDirectory is a mock of Directory interface.
type MockDirectory struct {
	ctrl     *gomock.Controller
	recorder *MockDirectoryMockRecorder
	isgomock struct{}
}

// MockDirectoryMockRecorder is the mock recorder for MockDirectory.
type MockDirectoryMockRecorder struct {
	mock *MockDirectory
}

// NewMockDirectory creates a new mock instance.
func NewMockDirectory(ctrl *gomock.Controller) *MockDirectory {
	mock := &MockDirectory{ctrl: ctrl}
	mock.recorder = &MockDirectoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDirectory) EXPECT() *MockDirectoryMockRecorder {
	return m.recorder
}

// LookupEndpoint mocks base method.
func (m *MockDirectory) LookupEndpoint(ctx context.Context, owner *fhir.Identifier, endpointName string) ([]fhir.Endpoint, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LookupEndpoint", ctx, owner, endpointName)
	ret0, _ := ret[0].([]fhir.Endpoint)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LookupEndpoint indicates an expected call of LookupEndpoint.
func (mr *MockDirectoryMockRecorder) LookupEndpoint(ctx, owner, endpointName any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LookupEndpoint", reflect.TypeOf((*MockDirectory)(nil).LookupEndpoint), ctx, owner, endpointName)
}

// LookupEntity mocks base method.
func (m *MockDirectory) LookupEntity(ctx context.Context, identifier fhir.Identifier) (*fhir.Reference, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LookupEntity", ctx, identifier)
	ret0, _ := ret[0].(*fhir.Reference)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LookupEntity indicates an expected call of LookupEntity.
func (mr *MockDirectoryMockRecorder) LookupEntity(ctx, identifier any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LookupEntity", reflect.TypeOf((*MockDirectory)(nil).LookupEntity), ctx, identifier)
}

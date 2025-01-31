// Code generated by MockGen. DO NOT EDIT.
// Source: client_wrapper.go
//
// Generated by this command:
//
//	mockgen -destination=./client_wrapper_mock.go -package=ehr -source=client_wrapper.go
//

// Package ehr is a generated GoMock package.
package ehr

import (
	context "context"
	reflect "reflect"

	azservicebus "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	gomock "go.uber.org/mock/gomock"
)

// MockSbClient is a mock of ServiceBusClientWrapper interface.
type MockSbClient struct {
	ctrl     *gomock.Controller
	recorder *MockSbClientMockRecorder
	isgomock struct{}
}

// MockSbClientMockRecorder is the mock recorder for MockSbClient.
type MockSbClientMockRecorder struct {
	mock *MockSbClient
}

// NewMockSbClient creates a new mock instance.
func NewMockSbClient(ctrl *gomock.Controller) *MockSbClient {
	mock := &MockSbClient{ctrl: ctrl}
	mock.recorder = &MockSbClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSbClient) EXPECT() *MockSbClientMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockSbClient) Close(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockSbClientMockRecorder) Close(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockSbClient)(nil).Close), ctx)
}

// SendMessage mocks base method.
func (m *MockSbClient) SendMessage(ctx context.Context, message *azservicebus.Message) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendMessage", ctx, message)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendMessage indicates an expected call of SendMessage.
func (mr *MockSbClientMockRecorder) SendMessage(ctx, message any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendMessage", reflect.TypeOf((*MockSbClient)(nil).SendMessage), ctx, message)
}

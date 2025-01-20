// Code generated by MockGen. DO NOT EDIT.
// Source: kafka_client.go
//
// Generated by this command:
//
//	mockgen -destination=./kafka_client_mock.go -package=ehr -source=kafka_client.go
//

// Package ehr is a generated GoMock package.
package ehr

import (
	context "context"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

// MockKafkaClient is a mock of KafkaClient interface.
type MockKafkaClient struct {
	ctrl     *gomock.Controller
	recorder *MockKafkaClientMockRecorder
	isgomock struct{}
}

// MockKafkaClientMockRecorder is the mock recorder for MockKafkaClient.
type MockKafkaClientMockRecorder struct {
	mock *MockKafkaClient
}

// NewMockKafkaClient creates a new mock instance.
func NewMockKafkaClient(ctrl *gomock.Controller) *MockKafkaClient {
	mock := &MockKafkaClient{ctrl: ctrl}
	mock.recorder = &MockKafkaClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockKafkaClient) EXPECT() *MockKafkaClientMockRecorder {
	return m.recorder
}

// PingConnection mocks base method.
func (m *MockKafkaClient) PingConnection(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PingConnection", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// PingConnection indicates an expected call of PingConnection.
func (mr *MockKafkaClientMockRecorder) PingConnection(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PingConnection", reflect.TypeOf((*MockKafkaClient)(nil).PingConnection), ctx)
}

// SubmitMessage mocks base method.
func (m *MockKafkaClient) SubmitMessage(ctx context.Context, key, value string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubmitMessage", ctx, key, value)
	ret0, _ := ret[0].(error)
	return ret0
}

// SubmitMessage indicates an expected call of SubmitMessage.
func (mr *MockKafkaClientMockRecorder) SubmitMessage(ctx, key, value any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubmitMessage", reflect.TypeOf((*MockKafkaClient)(nil).SubmitMessage), ctx, key, value)
}

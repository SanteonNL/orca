// Code generated by MockGen. DO NOT EDIT.
// Entity: service.go
//
// Generated by this command:
//
//	mockgen -destination=./service_mock.go -package=messaging -source=service.go
//

// Package messaging is a generated GoMock package.
package messaging

import (
	context "context"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

// MockBroker is a mock of Broker interface.
type MockBroker struct {
	ctrl     *gomock.Controller
	recorder *MockBrokerMockRecorder
	isgomock struct{}
}

// MockBrokerMockRecorder is the mock recorder for MockBroker.
type MockBrokerMockRecorder struct {
	mock *MockBroker
}

// NewMockBroker creates a new mock instance.
func NewMockBroker(ctrl *gomock.Controller) *MockBroker {
	mock := &MockBroker{ctrl: ctrl}
	mock.recorder = &MockBrokerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockBroker) EXPECT() *MockBrokerMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockBroker) Close(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockBrokerMockRecorder) Close(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockBroker)(nil).Close), ctx)
}

// Receive mocks base method.
func (m *MockBroker) Receive(queue string, handler func(context.Context, Message) error) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReceiveFromQueue", queue, handler)
	ret0, _ := ret[0].(error)
	return ret0
}

// Receive indicates an expected call of Receive.
func (mr *MockBrokerMockRecorder) Receive(queue, handler any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReceiveFromQueue", reflect.TypeOf((*MockBroker)(nil).Receive), queue, handler)
}

// SendMessage mocks base method.
func (m *MockBroker) SendMessage(ctx context.Context, topic string, message *Message) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendMessage", ctx, topic, message)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendMessage indicates an expected call of SendMessage.
func (mr *MockBrokerMockRecorder) SendMessage(ctx, topic, message any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendMessage", reflect.TypeOf((*MockBroker)(nil).SendMessage), ctx, topic, message)
}

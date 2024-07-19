// Code generated by mockery v2.43.2. DO NOT EDIT.

package fhirclient

import (
	fhirclient "github.com/SanteonNL/go-fhir-client"
	mock "github.com/stretchr/testify/mock"

	url "net/url"
)

// MockClient is an autogenerated mock type for the Client type
type MockClient struct {
	mock.Mock
}

type MockClient_Expecter struct {
	mock *mock.Mock
}

func (_m *MockClient) EXPECT() *MockClient_Expecter {
	return &MockClient_Expecter{mock: &_m.Mock}
}

// Create provides a mock function with given fields: resource, result, opts
func (_m *MockClient) Create(resource interface{}, result interface{}, opts ...fhirclient.Option) error {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, resource, result)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for Create")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(interface{}, interface{}, ...fhirclient.Option) error); ok {
		r0 = rf(resource, result, opts...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockClient_Create_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Create'
type MockClient_Create_Call struct {
	*mock.Call
}

// Create is a helper method to define mock.On call
//   - resource interface{}
//   - result interface{}
//   - opts ...fhirclient.Option
func (_e *MockClient_Expecter) Create(resource interface{}, result interface{}, opts ...interface{}) *MockClient_Create_Call {
	return &MockClient_Create_Call{Call: _e.mock.On("Create",
		append([]interface{}{resource, result}, opts...)...)}
}

func (_c *MockClient_Create_Call) Run(run func(resource interface{}, result interface{}, opts ...fhirclient.Option)) *MockClient_Create_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]fhirclient.Option, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(fhirclient.Option)
			}
		}
		run(args[0].(interface{}), args[1].(interface{}), variadicArgs...)
	})
	return _c
}

func (_c *MockClient_Create_Call) Return(_a0 error) *MockClient_Create_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockClient_Create_Call) RunAndReturn(run func(interface{}, interface{}, ...fhirclient.Option) error) *MockClient_Create_Call {
	_c.Call.Return(run)
	return _c
}

// Path provides a mock function with given fields: path
func (_m *MockClient) Path(path ...string) *url.URL {
	_va := make([]interface{}, len(path))
	for _i := range path {
		_va[_i] = path[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for Path")
	}

	var r0 *url.URL
	if rf, ok := ret.Get(0).(func(...string) *url.URL); ok {
		r0 = rf(path...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*url.URL)
		}
	}

	return r0
}

// MockClient_Path_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Path'
type MockClient_Path_Call struct {
	*mock.Call
}

// Path is a helper method to define mock.On call
//   - path ...string
func (_e *MockClient_Expecter) Path(path ...interface{}) *MockClient_Path_Call {
	return &MockClient_Path_Call{Call: _e.mock.On("Path",
		append([]interface{}{}, path...)...)}
}

func (_c *MockClient_Path_Call) Run(run func(path ...string)) *MockClient_Path_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]string, len(args)-0)
		for i, a := range args[0:] {
			if a != nil {
				variadicArgs[i] = a.(string)
			}
		}
		run(variadicArgs...)
	})
	return _c
}

func (_c *MockClient_Path_Call) Return(_a0 *url.URL) *MockClient_Path_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockClient_Path_Call) RunAndReturn(run func(...string) *url.URL) *MockClient_Path_Call {
	_c.Call.Return(run)
	return _c
}

// Read provides a mock function with given fields: path, target, opts
func (_m *MockClient) Read(path string, target interface{}, opts ...fhirclient.Option) error {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, path, target)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for Read")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string, interface{}, ...fhirclient.Option) error); ok {
		r0 = rf(path, target, opts...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockClient_Read_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Read'
type MockClient_Read_Call struct {
	*mock.Call
}

// Read is a helper method to define mock.On call
//   - path string
//   - target interface{}
//   - opts ...fhirclient.Option
func (_e *MockClient_Expecter) Read(path interface{}, target interface{}, opts ...interface{}) *MockClient_Read_Call {
	return &MockClient_Read_Call{Call: _e.mock.On("Read",
		append([]interface{}{path, target}, opts...)...)}
}

func (_c *MockClient_Read_Call) Run(run func(path string, target interface{}, opts ...fhirclient.Option)) *MockClient_Read_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]fhirclient.Option, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(fhirclient.Option)
			}
		}
		run(args[0].(string), args[1].(interface{}), variadicArgs...)
	})
	return _c
}

func (_c *MockClient_Read_Call) Return(_a0 error) *MockClient_Read_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockClient_Read_Call) RunAndReturn(run func(string, interface{}, ...fhirclient.Option) error) *MockClient_Read_Call {
	_c.Call.Return(run)
	return _c
}

// Update provides a mock function with given fields: path, resource, result, opts
func (_m *MockClient) Update(path string, resource interface{}, result interface{}, opts ...fhirclient.Option) error {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, path, resource, result)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for Update")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string, interface{}, interface{}, ...fhirclient.Option) error); ok {
		r0 = rf(path, resource, result, opts...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockClient_Update_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Update'
type MockClient_Update_Call struct {
	*mock.Call
}

// Update is a helper method to define mock.On call
//   - path string
//   - resource interface{}
//   - result interface{}
//   - opts ...fhirclient.Option
func (_e *MockClient_Expecter) Update(path interface{}, resource interface{}, result interface{}, opts ...interface{}) *MockClient_Update_Call {
	return &MockClient_Update_Call{Call: _e.mock.On("Update",
		append([]interface{}{path, resource, result}, opts...)...)}
}

func (_c *MockClient_Update_Call) Run(run func(path string, resource interface{}, result interface{}, opts ...fhirclient.Option)) *MockClient_Update_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]fhirclient.Option, len(args)-3)
		for i, a := range args[3:] {
			if a != nil {
				variadicArgs[i] = a.(fhirclient.Option)
			}
		}
		run(args[0].(string), args[1].(interface{}), args[2].(interface{}), variadicArgs...)
	})
	return _c
}

func (_c *MockClient_Update_Call) Return(_a0 error) *MockClient_Update_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockClient_Update_Call) RunAndReturn(run func(string, interface{}, interface{}, ...fhirclient.Option) error) *MockClient_Update_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockClient creates a new instance of MockClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockClient {
	mock := &MockClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

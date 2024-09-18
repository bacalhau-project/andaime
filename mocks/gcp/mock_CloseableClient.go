// Code generated by mockery v2.46.0. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"

// MockCloseableClient is an autogenerated mock type for the CloseableClient type
type MockCloseableClient struct {
	mock.Mock
}

type MockCloseableClient_Expecter struct {
	mock *mock.Mock
}

func (_m *MockCloseableClient) EXPECT() *MockCloseableClient_Expecter {
	return &MockCloseableClient_Expecter{mock: &_m.Mock}
}

// Close provides a mock function with given fields:
func (_m *MockCloseableClient) Close() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Close")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockCloseableClient_Close_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Close'
type MockCloseableClient_Close_Call struct {
	*mock.Call
}

// Close is a helper method to define mock.On call
func (_e *MockCloseableClient_Expecter) Close() *MockCloseableClient_Close_Call {
	return &MockCloseableClient_Close_Call{Call: _e.mock.On("Close")}
}

func (_c *MockCloseableClient_Close_Call) Run(run func()) *MockCloseableClient_Close_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockCloseableClient_Close_Call) Return(_a0 error) *MockCloseableClient_Close_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockCloseableClient_Close_Call) RunAndReturn(run func() error) *MockCloseableClient_Close_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockCloseableClient creates a new instance of MockCloseableClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockCloseableClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockCloseableClient {
	mock := &MockCloseableClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

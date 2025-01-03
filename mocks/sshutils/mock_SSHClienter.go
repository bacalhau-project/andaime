// Code generated by mockery v2.38.0. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"
	ssh "golang.org/x/crypto/ssh"

	sshutils "github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
)

// MockSSHClienter is an autogenerated mock type for the SSHClienter type
type MockSSHClienter struct {
	mock.Mock
}

type MockSSHClienter_Expecter struct {
	mock *mock.Mock
}

func (_m *MockSSHClienter) EXPECT() *MockSSHClienter_Expecter {
	return &MockSSHClienter_Expecter{mock: &_m.Mock}
}

// Close provides a mock function with given fields:
func (_m *MockSSHClienter) Close() error {
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

// MockSSHClienter_Close_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Close'
type MockSSHClienter_Close_Call struct {
	*mock.Call
}

// Close is a helper method to define mock.On call
func (_e *MockSSHClienter_Expecter) Close() *MockSSHClienter_Close_Call {
	return &MockSSHClienter_Close_Call{Call: _e.mock.On("Close")}
}

func (_c *MockSSHClienter_Close_Call) Run(run func()) *MockSSHClienter_Close_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockSSHClienter_Close_Call) Return(_a0 error) *MockSSHClienter_Close_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockSSHClienter_Close_Call) RunAndReturn(run func() error) *MockSSHClienter_Close_Call {
	_c.Call.Return(run)
	return _c
}

// Connect provides a mock function with given fields:
func (_m *MockSSHClienter) Connect() (sshutils.SSHClienter, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Connect")
	}

	var r0 sshutils.SSHClienter
	var r1 error
	if rf, ok := ret.Get(0).(func() (sshutils.SSHClienter, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() sshutils.SSHClienter); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(sshutils.SSHClienter)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSSHClienter_Connect_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Connect'
type MockSSHClienter_Connect_Call struct {
	*mock.Call
}

// Connect is a helper method to define mock.On call
func (_e *MockSSHClienter_Expecter) Connect() *MockSSHClienter_Connect_Call {
	return &MockSSHClienter_Connect_Call{Call: _e.mock.On("Connect")}
}

func (_c *MockSSHClienter_Connect_Call) Run(run func()) *MockSSHClienter_Connect_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockSSHClienter_Connect_Call) Return(_a0 sshutils.SSHClienter, _a1 error) *MockSSHClienter_Connect_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSSHClienter_Connect_Call) RunAndReturn(run func() (sshutils.SSHClienter, error)) *MockSSHClienter_Connect_Call {
	_c.Call.Return(run)
	return _c
}

// GetClient provides a mock function with given fields:
func (_m *MockSSHClienter) GetClient() *ssh.Client {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetClient")
	}

	var r0 *ssh.Client
	if rf, ok := ret.Get(0).(func() *ssh.Client); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*ssh.Client)
		}
	}

	return r0
}

// MockSSHClienter_GetClient_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetClient'
type MockSSHClienter_GetClient_Call struct {
	*mock.Call
}

// GetClient is a helper method to define mock.On call
func (_e *MockSSHClienter_Expecter) GetClient() *MockSSHClienter_GetClient_Call {
	return &MockSSHClienter_GetClient_Call{Call: _e.mock.On("GetClient")}
}

func (_c *MockSSHClienter_GetClient_Call) Run(run func()) *MockSSHClienter_GetClient_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockSSHClienter_GetClient_Call) Return(_a0 *ssh.Client) *MockSSHClienter_GetClient_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockSSHClienter_GetClient_Call) RunAndReturn(run func() *ssh.Client) *MockSSHClienter_GetClient_Call {
	_c.Call.Return(run)
	return _c
}

// IsConnected provides a mock function with given fields:
func (_m *MockSSHClienter) IsConnected() bool {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for IsConnected")
	}

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// MockSSHClienter_IsConnected_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'IsConnected'
type MockSSHClienter_IsConnected_Call struct {
	*mock.Call
}

// IsConnected is a helper method to define mock.On call
func (_e *MockSSHClienter_Expecter) IsConnected() *MockSSHClienter_IsConnected_Call {
	return &MockSSHClienter_IsConnected_Call{Call: _e.mock.On("IsConnected")}
}

func (_c *MockSSHClienter_IsConnected_Call) Run(run func()) *MockSSHClienter_IsConnected_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockSSHClienter_IsConnected_Call) Return(_a0 bool) *MockSSHClienter_IsConnected_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockSSHClienter_IsConnected_Call) RunAndReturn(run func() bool) *MockSSHClienter_IsConnected_Call {
	_c.Call.Return(run)
	return _c
}

// NewSession provides a mock function with given fields:
func (_m *MockSSHClienter) NewSession() (sshutils.SSHSessioner, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for NewSession")
	}

	var r0 sshutils.SSHSessioner
	var r1 error
	if rf, ok := ret.Get(0).(func() (sshutils.SSHSessioner, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() sshutils.SSHSessioner); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(sshutils.SSHSessioner)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSSHClienter_NewSession_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'NewSession'
type MockSSHClienter_NewSession_Call struct {
	*mock.Call
}

// NewSession is a helper method to define mock.On call
func (_e *MockSSHClienter_Expecter) NewSession() *MockSSHClienter_NewSession_Call {
	return &MockSSHClienter_NewSession_Call{Call: _e.mock.On("NewSession")}
}

func (_c *MockSSHClienter_NewSession_Call) Run(run func()) *MockSSHClienter_NewSession_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockSSHClienter_NewSession_Call) Return(_a0 sshutils.SSHSessioner, _a1 error) *MockSSHClienter_NewSession_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSSHClienter_NewSession_Call) RunAndReturn(run func() (sshutils.SSHSessioner, error)) *MockSSHClienter_NewSession_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockSSHClienter creates a new instance of MockSSHClienter. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockSSHClienter(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockSSHClienter {
	mock := &MockSSHClienter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

// Code generated by mockery v2.38.0. DO NOT EDIT.

package mocks

import (
	sftp "github.com/pkg/sftp"
	mock "github.com/stretchr/testify/mock"

	sshutils "github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
)

// MockSFTPClientCreator is an autogenerated mock type for the SFTPClientCreator type
type MockSFTPClientCreator struct {
	mock.Mock
}

type MockSFTPClientCreator_Expecter struct {
	mock *mock.Mock
}

func (_m *MockSFTPClientCreator) EXPECT() *MockSFTPClientCreator_Expecter {
	return &MockSFTPClientCreator_Expecter{mock: &_m.Mock}
}

// NewSFTPClient provides a mock function with given fields: client
func (_m *MockSFTPClientCreator) NewSFTPClient(client sshutils.SSHClienter) (*sftp.Client, error) {
	ret := _m.Called(client)

	if len(ret) == 0 {
		panic("no return value specified for NewSFTPClient")
	}

	var r0 *sftp.Client
	var r1 error
	if rf, ok := ret.Get(0).(func(sshutils.SSHClienter) (*sftp.Client, error)); ok {
		return rf(client)
	}
	if rf, ok := ret.Get(0).(func(sshutils.SSHClienter) *sftp.Client); ok {
		r0 = rf(client)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*sftp.Client)
		}
	}

	if rf, ok := ret.Get(1).(func(sshutils.SSHClienter) error); ok {
		r1 = rf(client)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSFTPClientCreator_NewSFTPClient_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'NewSFTPClient'
type MockSFTPClientCreator_NewSFTPClient_Call struct {
	*mock.Call
}

// NewSFTPClient is a helper method to define mock.On call
//   - client sshutils.SSHClienter
func (_e *MockSFTPClientCreator_Expecter) NewSFTPClient(client interface{}) *MockSFTPClientCreator_NewSFTPClient_Call {
	return &MockSFTPClientCreator_NewSFTPClient_Call{Call: _e.mock.On("NewSFTPClient", client)}
}

func (_c *MockSFTPClientCreator_NewSFTPClient_Call) Run(run func(client sshutils.SSHClienter)) *MockSFTPClientCreator_NewSFTPClient_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(sshutils.SSHClienter))
	})
	return _c
}

func (_c *MockSFTPClientCreator_NewSFTPClient_Call) Return(_a0 *sftp.Client, _a1 error) *MockSFTPClientCreator_NewSFTPClient_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSFTPClientCreator_NewSFTPClient_Call) RunAndReturn(run func(sshutils.SSHClienter) (*sftp.Client, error)) *MockSFTPClientCreator_NewSFTPClient_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockSFTPClientCreator creates a new instance of MockSFTPClientCreator. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockSFTPClientCreator(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockSFTPClientCreator {
	mock := &MockSFTPClientCreator{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

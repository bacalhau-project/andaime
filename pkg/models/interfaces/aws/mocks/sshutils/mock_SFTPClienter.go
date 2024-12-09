// Code generated by mockery v2.46.3. DO NOT EDIT.

package sshutils

import (
	io "io"
	fs "io/fs"

	mock "github.com/stretchr/testify/mock"
)

// MockSFTPClienter is an autogenerated mock type for the SFTPClienter type
type MockSFTPClienter struct {
	mock.Mock
}

type MockSFTPClienter_Expecter struct {
	mock *mock.Mock
}

func (_m *MockSFTPClienter) EXPECT() *MockSFTPClienter_Expecter {
	return &MockSFTPClienter_Expecter{mock: &_m.Mock}
}

// Chmod provides a mock function with given fields: _a0, _a1
func (_m *MockSFTPClienter) Chmod(_a0 string, _a1 fs.FileMode) error {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for Chmod")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string, fs.FileMode) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockSFTPClienter_Chmod_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Chmod'
type MockSFTPClienter_Chmod_Call struct {
	*mock.Call
}

// Chmod is a helper method to define mock.On call
//   - _a0 string
//   - _a1 fs.FileMode
func (_e *MockSFTPClienter_Expecter) Chmod(_a0 interface{}, _a1 interface{}) *MockSFTPClienter_Chmod_Call {
	return &MockSFTPClienter_Chmod_Call{Call: _e.mock.On("Chmod", _a0, _a1)}
}

func (_c *MockSFTPClienter_Chmod_Call) Run(run func(_a0 string, _a1 fs.FileMode)) *MockSFTPClienter_Chmod_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string), args[1].(fs.FileMode))
	})
	return _c
}

func (_c *MockSFTPClienter_Chmod_Call) Return(_a0 error) *MockSFTPClienter_Chmod_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockSFTPClienter_Chmod_Call) RunAndReturn(run func(string, fs.FileMode) error) *MockSFTPClienter_Chmod_Call {
	_c.Call.Return(run)
	return _c
}

// Close provides a mock function with given fields:
func (_m *MockSFTPClienter) Close() error {
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

// MockSFTPClienter_Close_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Close'
type MockSFTPClienter_Close_Call struct {
	*mock.Call
}

// Close is a helper method to define mock.On call
func (_e *MockSFTPClienter_Expecter) Close() *MockSFTPClienter_Close_Call {
	return &MockSFTPClienter_Close_Call{Call: _e.mock.On("Close")}
}

func (_c *MockSFTPClienter_Close_Call) Run(run func()) *MockSFTPClienter_Close_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockSFTPClienter_Close_Call) Return(_a0 error) *MockSFTPClienter_Close_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockSFTPClienter_Close_Call) RunAndReturn(run func() error) *MockSFTPClienter_Close_Call {
	_c.Call.Return(run)
	return _c
}

// Create provides a mock function with given fields: _a0
func (_m *MockSFTPClienter) Create(_a0 string) (io.WriteCloser, error) {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for Create")
	}

	var r0 io.WriteCloser
	var r1 error
	if rf, ok := ret.Get(0).(func(string) (io.WriteCloser, error)); ok {
		return rf(_a0)
	}
	if rf, ok := ret.Get(0).(func(string) io.WriteCloser); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(io.WriteCloser)
		}
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSFTPClienter_Create_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Create'
type MockSFTPClienter_Create_Call struct {
	*mock.Call
}

// Create is a helper method to define mock.On call
//   - _a0 string
func (_e *MockSFTPClienter_Expecter) Create(_a0 interface{}) *MockSFTPClienter_Create_Call {
	return &MockSFTPClienter_Create_Call{Call: _e.mock.On("Create", _a0)}
}

func (_c *MockSFTPClienter_Create_Call) Run(run func(_a0 string)) *MockSFTPClienter_Create_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *MockSFTPClienter_Create_Call) Return(_a0 io.WriteCloser, _a1 error) *MockSFTPClienter_Create_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSFTPClienter_Create_Call) RunAndReturn(run func(string) (io.WriteCloser, error)) *MockSFTPClienter_Create_Call {
	_c.Call.Return(run)
	return _c
}

// MkdirAll provides a mock function with given fields: _a0
func (_m *MockSFTPClienter) MkdirAll(_a0 string) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for MkdirAll")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockSFTPClienter_MkdirAll_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'MkdirAll'
type MockSFTPClienter_MkdirAll_Call struct {
	*mock.Call
}

// MkdirAll is a helper method to define mock.On call
//   - _a0 string
func (_e *MockSFTPClienter_Expecter) MkdirAll(_a0 interface{}) *MockSFTPClienter_MkdirAll_Call {
	return &MockSFTPClienter_MkdirAll_Call{Call: _e.mock.On("MkdirAll", _a0)}
}

func (_c *MockSFTPClienter_MkdirAll_Call) Run(run func(_a0 string)) *MockSFTPClienter_MkdirAll_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *MockSFTPClienter_MkdirAll_Call) Return(_a0 error) *MockSFTPClienter_MkdirAll_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockSFTPClienter_MkdirAll_Call) RunAndReturn(run func(string) error) *MockSFTPClienter_MkdirAll_Call {
	_c.Call.Return(run)
	return _c
}

// Open provides a mock function with given fields: _a0
func (_m *MockSFTPClienter) Open(_a0 string) (io.ReadCloser, error) {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for Open")
	}

	var r0 io.ReadCloser
	var r1 error
	if rf, ok := ret.Get(0).(func(string) (io.ReadCloser, error)); ok {
		return rf(_a0)
	}
	if rf, ok := ret.Get(0).(func(string) io.ReadCloser); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(io.ReadCloser)
		}
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSFTPClienter_Open_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Open'
type MockSFTPClienter_Open_Call struct {
	*mock.Call
}

// Open is a helper method to define mock.On call
//   - _a0 string
func (_e *MockSFTPClienter_Expecter) Open(_a0 interface{}) *MockSFTPClienter_Open_Call {
	return &MockSFTPClienter_Open_Call{Call: _e.mock.On("Open", _a0)}
}

func (_c *MockSFTPClienter_Open_Call) Run(run func(_a0 string)) *MockSFTPClienter_Open_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *MockSFTPClienter_Open_Call) Return(_a0 io.ReadCloser, _a1 error) *MockSFTPClienter_Open_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSFTPClienter_Open_Call) RunAndReturn(run func(string) (io.ReadCloser, error)) *MockSFTPClienter_Open_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockSFTPClienter creates a new instance of MockSFTPClienter. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockSFTPClienter(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockSFTPClienter {
	mock := &MockSFTPClienter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
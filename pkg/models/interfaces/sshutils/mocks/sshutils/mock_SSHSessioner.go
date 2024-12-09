// Code generated by mockery v2.46.3. DO NOT EDIT.

package sshutils

import (
	io "io"

	mock "github.com/stretchr/testify/mock"
	ssh "golang.org/x/crypto/ssh"
)

// MockSSHSessioner is an autogenerated mock type for the SSHSessioner type
type MockSSHSessioner struct {
	mock.Mock
}

type MockSSHSessioner_Expecter struct {
	mock *mock.Mock
}

func (_m *MockSSHSessioner) EXPECT() *MockSSHSessioner_Expecter {
	return &MockSSHSessioner_Expecter{mock: &_m.Mock}
}

// Close provides a mock function with given fields:
func (_m *MockSSHSessioner) Close() error {
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

// MockSSHSessioner_Close_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Close'
type MockSSHSessioner_Close_Call struct {
	*mock.Call
}

// Close is a helper method to define mock.On call
func (_e *MockSSHSessioner_Expecter) Close() *MockSSHSessioner_Close_Call {
	return &MockSSHSessioner_Close_Call{Call: _e.mock.On("Close")}
}

func (_c *MockSSHSessioner_Close_Call) Run(run func()) *MockSSHSessioner_Close_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockSSHSessioner_Close_Call) Return(_a0 error) *MockSSHSessioner_Close_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockSSHSessioner_Close_Call) RunAndReturn(run func() error) *MockSSHSessioner_Close_Call {
	_c.Call.Return(run)
	return _c
}

// CombinedOutput provides a mock function with given fields: cmd
func (_m *MockSSHSessioner) CombinedOutput(cmd string) ([]byte, error) {
	ret := _m.Called(cmd)

	if len(ret) == 0 {
		panic("no return value specified for CombinedOutput")
	}

	var r0 []byte
	var r1 error
	if rf, ok := ret.Get(0).(func(string) ([]byte, error)); ok {
		return rf(cmd)
	}
	if rf, ok := ret.Get(0).(func(string) []byte); ok {
		r0 = rf(cmd)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(cmd)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSSHSessioner_CombinedOutput_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CombinedOutput'
type MockSSHSessioner_CombinedOutput_Call struct {
	*mock.Call
}

// CombinedOutput is a helper method to define mock.On call
//   - cmd string
func (_e *MockSSHSessioner_Expecter) CombinedOutput(cmd interface{}) *MockSSHSessioner_CombinedOutput_Call {
	return &MockSSHSessioner_CombinedOutput_Call{Call: _e.mock.On("CombinedOutput", cmd)}
}

func (_c *MockSSHSessioner_CombinedOutput_Call) Run(run func(cmd string)) *MockSSHSessioner_CombinedOutput_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *MockSSHSessioner_CombinedOutput_Call) Return(_a0 []byte, _a1 error) *MockSSHSessioner_CombinedOutput_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSSHSessioner_CombinedOutput_Call) RunAndReturn(run func(string) ([]byte, error)) *MockSSHSessioner_CombinedOutput_Call {
	_c.Call.Return(run)
	return _c
}

// Run provides a mock function with given fields: cmd
func (_m *MockSSHSessioner) Run(cmd string) error {
	ret := _m.Called(cmd)

	if len(ret) == 0 {
		panic("no return value specified for Run")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(cmd)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockSSHSessioner_Run_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Run'
type MockSSHSessioner_Run_Call struct {
	*mock.Call
}

// Run is a helper method to define mock.On call
//   - cmd string
func (_e *MockSSHSessioner_Expecter) Run(cmd interface{}) *MockSSHSessioner_Run_Call {
	return &MockSSHSessioner_Run_Call{Call: _e.mock.On("Run", cmd)}
}

func (_c *MockSSHSessioner_Run_Call) Run(run func(cmd string)) *MockSSHSessioner_Run_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *MockSSHSessioner_Run_Call) Return(_a0 error) *MockSSHSessioner_Run_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockSSHSessioner_Run_Call) RunAndReturn(run func(string) error) *MockSSHSessioner_Run_Call {
	_c.Call.Return(run)
	return _c
}

// SetStderr provides a mock function with given fields: _a0
func (_m *MockSSHSessioner) SetStderr(_a0 io.Writer) {
	_m.Called(_a0)
}

// MockSSHSessioner_SetStderr_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SetStderr'
type MockSSHSessioner_SetStderr_Call struct {
	*mock.Call
}

// SetStderr is a helper method to define mock.On call
//   - _a0 io.Writer
func (_e *MockSSHSessioner_Expecter) SetStderr(_a0 interface{}) *MockSSHSessioner_SetStderr_Call {
	return &MockSSHSessioner_SetStderr_Call{Call: _e.mock.On("SetStderr", _a0)}
}

func (_c *MockSSHSessioner_SetStderr_Call) Run(run func(_a0 io.Writer)) *MockSSHSessioner_SetStderr_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(io.Writer))
	})
	return _c
}

func (_c *MockSSHSessioner_SetStderr_Call) Return() *MockSSHSessioner_SetStderr_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockSSHSessioner_SetStderr_Call) RunAndReturn(run func(io.Writer)) *MockSSHSessioner_SetStderr_Call {
	_c.Call.Return(run)
	return _c
}

// SetStdout provides a mock function with given fields: _a0
func (_m *MockSSHSessioner) SetStdout(_a0 io.Writer) {
	_m.Called(_a0)
}

// MockSSHSessioner_SetStdout_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SetStdout'
type MockSSHSessioner_SetStdout_Call struct {
	*mock.Call
}

// SetStdout is a helper method to define mock.On call
//   - _a0 io.Writer
func (_e *MockSSHSessioner_Expecter) SetStdout(_a0 interface{}) *MockSSHSessioner_SetStdout_Call {
	return &MockSSHSessioner_SetStdout_Call{Call: _e.mock.On("SetStdout", _a0)}
}

func (_c *MockSSHSessioner_SetStdout_Call) Run(run func(_a0 io.Writer)) *MockSSHSessioner_SetStdout_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(io.Writer))
	})
	return _c
}

func (_c *MockSSHSessioner_SetStdout_Call) Return() *MockSSHSessioner_SetStdout_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockSSHSessioner_SetStdout_Call) RunAndReturn(run func(io.Writer)) *MockSSHSessioner_SetStdout_Call {
	_c.Call.Return(run)
	return _c
}

// Signal provides a mock function with given fields: sig
func (_m *MockSSHSessioner) Signal(sig ssh.Signal) error {
	ret := _m.Called(sig)

	if len(ret) == 0 {
		panic("no return value specified for Signal")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(ssh.Signal) error); ok {
		r0 = rf(sig)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockSSHSessioner_Signal_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Signal'
type MockSSHSessioner_Signal_Call struct {
	*mock.Call
}

// Signal is a helper method to define mock.On call
//   - sig ssh.Signal
func (_e *MockSSHSessioner_Expecter) Signal(sig interface{}) *MockSSHSessioner_Signal_Call {
	return &MockSSHSessioner_Signal_Call{Call: _e.mock.On("Signal", sig)}
}

func (_c *MockSSHSessioner_Signal_Call) Run(run func(sig ssh.Signal)) *MockSSHSessioner_Signal_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(ssh.Signal))
	})
	return _c
}

func (_c *MockSSHSessioner_Signal_Call) Return(_a0 error) *MockSSHSessioner_Signal_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockSSHSessioner_Signal_Call) RunAndReturn(run func(ssh.Signal) error) *MockSSHSessioner_Signal_Call {
	_c.Call.Return(run)
	return _c
}

// Start provides a mock function with given fields: cmd
func (_m *MockSSHSessioner) Start(cmd string) error {
	ret := _m.Called(cmd)

	if len(ret) == 0 {
		panic("no return value specified for Start")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(cmd)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockSSHSessioner_Start_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Start'
type MockSSHSessioner_Start_Call struct {
	*mock.Call
}

// Start is a helper method to define mock.On call
//   - cmd string
func (_e *MockSSHSessioner_Expecter) Start(cmd interface{}) *MockSSHSessioner_Start_Call {
	return &MockSSHSessioner_Start_Call{Call: _e.mock.On("Start", cmd)}
}

func (_c *MockSSHSessioner_Start_Call) Run(run func(cmd string)) *MockSSHSessioner_Start_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *MockSSHSessioner_Start_Call) Return(_a0 error) *MockSSHSessioner_Start_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockSSHSessioner_Start_Call) RunAndReturn(run func(string) error) *MockSSHSessioner_Start_Call {
	_c.Call.Return(run)
	return _c
}

// StdinPipe provides a mock function with given fields:
func (_m *MockSSHSessioner) StdinPipe() (io.WriteCloser, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for StdinPipe")
	}

	var r0 io.WriteCloser
	var r1 error
	if rf, ok := ret.Get(0).(func() (io.WriteCloser, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() io.WriteCloser); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(io.WriteCloser)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSSHSessioner_StdinPipe_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'StdinPipe'
type MockSSHSessioner_StdinPipe_Call struct {
	*mock.Call
}

// StdinPipe is a helper method to define mock.On call
func (_e *MockSSHSessioner_Expecter) StdinPipe() *MockSSHSessioner_StdinPipe_Call {
	return &MockSSHSessioner_StdinPipe_Call{Call: _e.mock.On("StdinPipe")}
}

func (_c *MockSSHSessioner_StdinPipe_Call) Run(run func()) *MockSSHSessioner_StdinPipe_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockSSHSessioner_StdinPipe_Call) Return(_a0 io.WriteCloser, _a1 error) *MockSSHSessioner_StdinPipe_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSSHSessioner_StdinPipe_Call) RunAndReturn(run func() (io.WriteCloser, error)) *MockSSHSessioner_StdinPipe_Call {
	_c.Call.Return(run)
	return _c
}

// Wait provides a mock function with given fields:
func (_m *MockSSHSessioner) Wait() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Wait")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockSSHSessioner_Wait_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Wait'
type MockSSHSessioner_Wait_Call struct {
	*mock.Call
}

// Wait is a helper method to define mock.On call
func (_e *MockSSHSessioner_Expecter) Wait() *MockSSHSessioner_Wait_Call {
	return &MockSSHSessioner_Wait_Call{Call: _e.mock.On("Wait")}
}

func (_c *MockSSHSessioner_Wait_Call) Run(run func()) *MockSSHSessioner_Wait_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockSSHSessioner_Wait_Call) Return(_a0 error) *MockSSHSessioner_Wait_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockSSHSessioner_Wait_Call) RunAndReturn(run func() error) *MockSSHSessioner_Wait_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockSSHSessioner creates a new instance of MockSSHSessioner. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockSSHSessioner(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockSSHSessioner {
	mock := &MockSSHSessioner{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
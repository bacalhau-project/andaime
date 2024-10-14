// Code generated by mockery v2.46.2. DO NOT EDIT.

package mocks

import (
	context "context"

	models "github.com/bacalhau-project/andaime/pkg/models"
	mock "github.com/stretchr/testify/mock"

	sshutils "github.com/bacalhau-project/andaime/pkg/sshutils"
)

// MockClusterDeployerer is an autogenerated mock type for the ClusterDeployerer type
type MockClusterDeployerer struct {
	mock.Mock
}

type MockClusterDeployerer_Expecter struct {
	mock *mock.Mock
}

func (_m *MockClusterDeployerer) EXPECT() *MockClusterDeployerer_Expecter {
	return &MockClusterDeployerer_Expecter{mock: &_m.Mock}
}

// ApplyBacalhauConfigs provides a mock function with given fields: ctx, sshConfig, bacalhauSettings
func (_m *MockClusterDeployerer) ApplyBacalhauConfigs(ctx context.Context, sshConfig sshutils.SSHConfiger, bacalhauSettings []models.BacalhauSettings) error {
	ret := _m.Called(ctx, sshConfig, bacalhauSettings)

	if len(ret) == 0 {
		panic("no return value specified for ApplyBacalhauConfigs")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, sshutils.SSHConfiger, []models.BacalhauSettings) error); ok {
		r0 = rf(ctx, sshConfig, bacalhauSettings)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockClusterDeployerer_ApplyBacalhauConfigs_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ApplyBacalhauConfigs'
type MockClusterDeployerer_ApplyBacalhauConfigs_Call struct {
	*mock.Call
}

// ApplyBacalhauConfigs is a helper method to define mock.On call
//   - ctx context.Context
//   - sshConfig sshutils.SSHConfiger
//   - bacalhauSettings []models.BacalhauSettings
func (_e *MockClusterDeployerer_Expecter) ApplyBacalhauConfigs(ctx interface{}, sshConfig interface{}, bacalhauSettings interface{}) *MockClusterDeployerer_ApplyBacalhauConfigs_Call {
	return &MockClusterDeployerer_ApplyBacalhauConfigs_Call{Call: _e.mock.On("ApplyBacalhauConfigs", ctx, sshConfig, bacalhauSettings)}
}

func (_c *MockClusterDeployerer_ApplyBacalhauConfigs_Call) Run(run func(ctx context.Context, sshConfig sshutils.SSHConfiger, bacalhauSettings []models.BacalhauSettings)) *MockClusterDeployerer_ApplyBacalhauConfigs_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(sshutils.SSHConfiger), args[2].([]models.BacalhauSettings))
	})
	return _c
}

func (_c *MockClusterDeployerer_ApplyBacalhauConfigs_Call) Return(_a0 error) *MockClusterDeployerer_ApplyBacalhauConfigs_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockClusterDeployerer_ApplyBacalhauConfigs_Call) RunAndReturn(run func(context.Context, sshutils.SSHConfiger, []models.BacalhauSettings) error) *MockClusterDeployerer_ApplyBacalhauConfigs_Call {
	_c.Call.Return(run)
	return _c
}

// ExecuteCustomScript provides a mock function with given fields: ctx, sshConfig, machine
func (_m *MockClusterDeployerer) ExecuteCustomScript(ctx context.Context, sshConfig sshutils.SSHConfiger, machine models.Machiner) error {
	ret := _m.Called(ctx, sshConfig, machine)

	if len(ret) == 0 {
		panic("no return value specified for ExecuteCustomScript")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, sshutils.SSHConfiger, models.Machiner) error); ok {
		r0 = rf(ctx, sshConfig, machine)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockClusterDeployerer_ExecuteCustomScript_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ExecuteCustomScript'
type MockClusterDeployerer_ExecuteCustomScript_Call struct {
	*mock.Call
}

// ExecuteCustomScript is a helper method to define mock.On call
//   - ctx context.Context
//   - sshConfig sshutils.SSHConfiger
//   - machine models.Machiner
func (_e *MockClusterDeployerer_Expecter) ExecuteCustomScript(ctx interface{}, sshConfig interface{}, machine interface{}) *MockClusterDeployerer_ExecuteCustomScript_Call {
	return &MockClusterDeployerer_ExecuteCustomScript_Call{Call: _e.mock.On("ExecuteCustomScript", ctx, sshConfig, machine)}
}

func (_c *MockClusterDeployerer_ExecuteCustomScript_Call) Run(run func(ctx context.Context, sshConfig sshutils.SSHConfiger, machine models.Machiner)) *MockClusterDeployerer_ExecuteCustomScript_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(sshutils.SSHConfiger), args[2].(models.Machiner))
	})
	return _c
}

func (_c *MockClusterDeployerer_ExecuteCustomScript_Call) Return(_a0 error) *MockClusterDeployerer_ExecuteCustomScript_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockClusterDeployerer_ExecuteCustomScript_Call) RunAndReturn(run func(context.Context, sshutils.SSHConfiger, models.Machiner) error) *MockClusterDeployerer_ExecuteCustomScript_Call {
	_c.Call.Return(run)
	return _c
}

// ProvisionBacalhauCluster provides a mock function with given fields: ctx
func (_m *MockClusterDeployerer) ProvisionBacalhauCluster(ctx context.Context) error {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for ProvisionBacalhauCluster")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockClusterDeployerer_ProvisionBacalhauCluster_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ProvisionBacalhauCluster'
type MockClusterDeployerer_ProvisionBacalhauCluster_Call struct {
	*mock.Call
}

// ProvisionBacalhauCluster is a helper method to define mock.On call
//   - ctx context.Context
func (_e *MockClusterDeployerer_Expecter) ProvisionBacalhauCluster(ctx interface{}) *MockClusterDeployerer_ProvisionBacalhauCluster_Call {
	return &MockClusterDeployerer_ProvisionBacalhauCluster_Call{Call: _e.mock.On("ProvisionBacalhauCluster", ctx)}
}

func (_c *MockClusterDeployerer_ProvisionBacalhauCluster_Call) Run(run func(ctx context.Context)) *MockClusterDeployerer_ProvisionBacalhauCluster_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockClusterDeployerer_ProvisionBacalhauCluster_Call) Return(_a0 error) *MockClusterDeployerer_ProvisionBacalhauCluster_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockClusterDeployerer_ProvisionBacalhauCluster_Call) RunAndReturn(run func(context.Context) error) *MockClusterDeployerer_ProvisionBacalhauCluster_Call {
	_c.Call.Return(run)
	return _c
}

// ProvisionMachine provides a mock function with given fields: ctx, sshConfig, machine
func (_m *MockClusterDeployerer) ProvisionMachine(ctx context.Context, sshConfig sshutils.SSHConfiger, machine models.Machiner) error {
	ret := _m.Called(ctx, sshConfig, machine)

	if len(ret) == 0 {
		panic("no return value specified for ProvisionMachine")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, sshutils.SSHConfiger, models.Machiner) error); ok {
		r0 = rf(ctx, sshConfig, machine)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockClusterDeployerer_ProvisionMachine_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ProvisionMachine'
type MockClusterDeployerer_ProvisionMachine_Call struct {
	*mock.Call
}

// ProvisionMachine is a helper method to define mock.On call
//   - ctx context.Context
//   - sshConfig sshutils.SSHConfiger
//   - machine models.Machiner
func (_e *MockClusterDeployerer_Expecter) ProvisionMachine(ctx interface{}, sshConfig interface{}, machine interface{}) *MockClusterDeployerer_ProvisionMachine_Call {
	return &MockClusterDeployerer_ProvisionMachine_Call{Call: _e.mock.On("ProvisionMachine", ctx, sshConfig, machine)}
}

func (_c *MockClusterDeployerer_ProvisionMachine_Call) Run(run func(ctx context.Context, sshConfig sshutils.SSHConfiger, machine models.Machiner)) *MockClusterDeployerer_ProvisionMachine_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(sshutils.SSHConfiger), args[2].(models.Machiner))
	})
	return _c
}

func (_c *MockClusterDeployerer_ProvisionMachine_Call) Return(_a0 error) *MockClusterDeployerer_ProvisionMachine_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockClusterDeployerer_ProvisionMachine_Call) RunAndReturn(run func(context.Context, sshutils.SSHConfiger, models.Machiner) error) *MockClusterDeployerer_ProvisionMachine_Call {
	_c.Call.Return(run)
	return _c
}

// ProvisionOrchestrator provides a mock function with given fields: ctx, machineName
func (_m *MockClusterDeployerer) ProvisionOrchestrator(ctx context.Context, machineName string) error {
	ret := _m.Called(ctx, machineName)

	if len(ret) == 0 {
		panic("no return value specified for ProvisionOrchestrator")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, machineName)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockClusterDeployerer_ProvisionOrchestrator_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ProvisionOrchestrator'
type MockClusterDeployerer_ProvisionOrchestrator_Call struct {
	*mock.Call
}

// ProvisionOrchestrator is a helper method to define mock.On call
//   - ctx context.Context
//   - machineName string
func (_e *MockClusterDeployerer_Expecter) ProvisionOrchestrator(ctx interface{}, machineName interface{}) *MockClusterDeployerer_ProvisionOrchestrator_Call {
	return &MockClusterDeployerer_ProvisionOrchestrator_Call{Call: _e.mock.On("ProvisionOrchestrator", ctx, machineName)}
}

func (_c *MockClusterDeployerer_ProvisionOrchestrator_Call) Run(run func(ctx context.Context, machineName string)) *MockClusterDeployerer_ProvisionOrchestrator_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockClusterDeployerer_ProvisionOrchestrator_Call) Return(_a0 error) *MockClusterDeployerer_ProvisionOrchestrator_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockClusterDeployerer_ProvisionOrchestrator_Call) RunAndReturn(run func(context.Context, string) error) *MockClusterDeployerer_ProvisionOrchestrator_Call {
	_c.Call.Return(run)
	return _c
}

// ProvisionWorker provides a mock function with given fields: ctx, machineName
func (_m *MockClusterDeployerer) ProvisionWorker(ctx context.Context, machineName string) error {
	ret := _m.Called(ctx, machineName)

	if len(ret) == 0 {
		panic("no return value specified for ProvisionWorker")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, machineName)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockClusterDeployerer_ProvisionWorker_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ProvisionWorker'
type MockClusterDeployerer_ProvisionWorker_Call struct {
	*mock.Call
}

// ProvisionWorker is a helper method to define mock.On call
//   - ctx context.Context
//   - machineName string
func (_e *MockClusterDeployerer_Expecter) ProvisionWorker(ctx interface{}, machineName interface{}) *MockClusterDeployerer_ProvisionWorker_Call {
	return &MockClusterDeployerer_ProvisionWorker_Call{Call: _e.mock.On("ProvisionWorker", ctx, machineName)}
}

func (_c *MockClusterDeployerer_ProvisionWorker_Call) Run(run func(ctx context.Context, machineName string)) *MockClusterDeployerer_ProvisionWorker_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockClusterDeployerer_ProvisionWorker_Call) Return(_a0 error) *MockClusterDeployerer_ProvisionWorker_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockClusterDeployerer_ProvisionWorker_Call) RunAndReturn(run func(context.Context, string) error) *MockClusterDeployerer_ProvisionWorker_Call {
	_c.Call.Return(run)
	return _c
}

// WaitForAllMachinesToReachState provides a mock function with given fields: ctx, resourceType, state
func (_m *MockClusterDeployerer) WaitForAllMachinesToReachState(ctx context.Context, resourceType string, state models.MachineResourceState) error {
	ret := _m.Called(ctx, resourceType, state)

	if len(ret) == 0 {
		panic("no return value specified for WaitForAllMachinesToReachState")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, models.MachineResourceState) error); ok {
		r0 = rf(ctx, resourceType, state)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockClusterDeployerer_WaitForAllMachinesToReachState_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'WaitForAllMachinesToReachState'
type MockClusterDeployerer_WaitForAllMachinesToReachState_Call struct {
	*mock.Call
}

// WaitForAllMachinesToReachState is a helper method to define mock.On call
//   - ctx context.Context
//   - resourceType string
//   - state models.MachineResourceState
func (_e *MockClusterDeployerer_Expecter) WaitForAllMachinesToReachState(ctx interface{}, resourceType interface{}, state interface{}) *MockClusterDeployerer_WaitForAllMachinesToReachState_Call {
	return &MockClusterDeployerer_WaitForAllMachinesToReachState_Call{Call: _e.mock.On("WaitForAllMachinesToReachState", ctx, resourceType, state)}
}

func (_c *MockClusterDeployerer_WaitForAllMachinesToReachState_Call) Run(run func(ctx context.Context, resourceType string, state models.MachineResourceState)) *MockClusterDeployerer_WaitForAllMachinesToReachState_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(models.MachineResourceState))
	})
	return _c
}

func (_c *MockClusterDeployerer_WaitForAllMachinesToReachState_Call) Return(_a0 error) *MockClusterDeployerer_WaitForAllMachinesToReachState_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockClusterDeployerer_WaitForAllMachinesToReachState_Call) RunAndReturn(run func(context.Context, string, models.MachineResourceState) error) *MockClusterDeployerer_WaitForAllMachinesToReachState_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockClusterDeployerer creates a new instance of MockClusterDeployerer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockClusterDeployerer(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockClusterDeployerer {
	mock := &MockClusterDeployerer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

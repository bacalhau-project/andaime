// Code generated by mockery v2.46.3. DO NOT EDIT.

package mocks

import (
	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	aws "github.com/bacalhau-project/andaime/pkg/models/interfaces/aws"

	awsec2 "github.com/aws/aws-cdk-go/awscdk/v2/awsec2"

	mock "github.com/stretchr/testify/mock"
)

// MockAWSInfraProviderer is an autogenerated mock type for the AWSInfraProviderer type
type MockAWSInfraProviderer struct {
	mock.Mock
}

type MockAWSInfraProviderer_Expecter struct {
	mock *mock.Mock
}

func (_m *MockAWSInfraProviderer) EXPECT() *MockAWSInfraProviderer_Expecter {
	return &MockAWSInfraProviderer_Expecter{mock: &_m.Mock}
}

// CreateVPC provides a mock function with given fields: stack
func (_m *MockAWSInfraProviderer) CreateVPC(stack awscdk.Stack) awsec2.IVpc {
	ret := _m.Called(stack)

	if len(ret) == 0 {
		panic("no return value specified for CreateVPC")
	}

	var r0 awsec2.IVpc
	if rf, ok := ret.Get(0).(func(awscdk.Stack) awsec2.IVpc); ok {
		r0 = rf(stack)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(awsec2.IVpc)
		}
	}

	return r0
}

// MockAWSInfraProviderer_CreateVPC_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CreateVPC'
type MockAWSInfraProviderer_CreateVPC_Call struct {
	*mock.Call
}

// CreateVPC is a helper method to define mock.On call
//   - stack awscdk.Stack
func (_e *MockAWSInfraProviderer_Expecter) CreateVPC(stack interface{}) *MockAWSInfraProviderer_CreateVPC_Call {
	return &MockAWSInfraProviderer_CreateVPC_Call{Call: _e.mock.On("CreateVPC", stack)}
}

func (_c *MockAWSInfraProviderer_CreateVPC_Call) Run(run func(stack awscdk.Stack)) *MockAWSInfraProviderer_CreateVPC_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(awscdk.Stack))
	})
	return _c
}

func (_c *MockAWSInfraProviderer_CreateVPC_Call) Return(_a0 awsec2.IVpc) *MockAWSInfraProviderer_CreateVPC_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockAWSInfraProviderer_CreateVPC_Call) RunAndReturn(run func(awscdk.Stack) awsec2.IVpc) *MockAWSInfraProviderer_CreateVPC_Call {
	_c.Call.Return(run)
	return _c
}

// GetCloudFormationClient provides a mock function with given fields:
func (_m *MockAWSInfraProviderer) GetCloudFormationClient() aws.CloudFormationAPIer {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetCloudFormationClient")
	}

	var r0 aws.CloudFormationAPIer
	if rf, ok := ret.Get(0).(func() aws.CloudFormationAPIer); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(aws.CloudFormationAPIer)
		}
	}

	return r0
}

// MockAWSInfraProviderer_GetCloudFormationClient_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetCloudFormationClient'
type MockAWSInfraProviderer_GetCloudFormationClient_Call struct {
	*mock.Call
}

// GetCloudFormationClient is a helper method to define mock.On call
func (_e *MockAWSInfraProviderer_Expecter) GetCloudFormationClient() *MockAWSInfraProviderer_GetCloudFormationClient_Call {
	return &MockAWSInfraProviderer_GetCloudFormationClient_Call{Call: _e.mock.On("GetCloudFormationClient")}
}

func (_c *MockAWSInfraProviderer_GetCloudFormationClient_Call) Run(run func()) *MockAWSInfraProviderer_GetCloudFormationClient_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockAWSInfraProviderer_GetCloudFormationClient_Call) Return(_a0 aws.CloudFormationAPIer) *MockAWSInfraProviderer_GetCloudFormationClient_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockAWSInfraProviderer_GetCloudFormationClient_Call) RunAndReturn(run func() aws.CloudFormationAPIer) *MockAWSInfraProviderer_GetCloudFormationClient_Call {
	_c.Call.Return(run)
	return _c
}

// NewStack provides a mock function with given fields: scope, id, props
func (_m *MockAWSInfraProviderer) NewStack(scope awscdk.App, id string, props *awscdk.StackProps) awscdk.Stack {
	ret := _m.Called(scope, id, props)

	if len(ret) == 0 {
		panic("no return value specified for NewStack")
	}

	var r0 awscdk.Stack
	if rf, ok := ret.Get(0).(func(awscdk.App, string, *awscdk.StackProps) awscdk.Stack); ok {
		r0 = rf(scope, id, props)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(awscdk.Stack)
		}
	}

	return r0
}

// MockAWSInfraProviderer_NewStack_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'NewStack'
type MockAWSInfraProviderer_NewStack_Call struct {
	*mock.Call
}

// NewStack is a helper method to define mock.On call
//   - scope awscdk.App
//   - id string
//   - props *awscdk.StackProps
func (_e *MockAWSInfraProviderer_Expecter) NewStack(scope interface{}, id interface{}, props interface{}) *MockAWSInfraProviderer_NewStack_Call {
	return &MockAWSInfraProviderer_NewStack_Call{Call: _e.mock.On("NewStack", scope, id, props)}
}

func (_c *MockAWSInfraProviderer_NewStack_Call) Run(run func(scope awscdk.App, id string, props *awscdk.StackProps)) *MockAWSInfraProviderer_NewStack_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(awscdk.App), args[1].(string), args[2].(*awscdk.StackProps))
	})
	return _c
}

func (_c *MockAWSInfraProviderer_NewStack_Call) Return(_a0 awscdk.Stack) *MockAWSInfraProviderer_NewStack_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockAWSInfraProviderer_NewStack_Call) RunAndReturn(run func(awscdk.App, string, *awscdk.StackProps) awscdk.Stack) *MockAWSInfraProviderer_NewStack_Call {
	_c.Call.Return(run)
	return _c
}

// SynthTemplate provides a mock function with given fields: app
func (_m *MockAWSInfraProviderer) SynthTemplate(app awscdk.App) (string, error) {
	ret := _m.Called(app)

	if len(ret) == 0 {
		panic("no return value specified for SynthTemplate")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func(awscdk.App) (string, error)); ok {
		return rf(app)
	}
	if rf, ok := ret.Get(0).(func(awscdk.App) string); ok {
		r0 = rf(app)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(awscdk.App) error); ok {
		r1 = rf(app)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockAWSInfraProviderer_SynthTemplate_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SynthTemplate'
type MockAWSInfraProviderer_SynthTemplate_Call struct {
	*mock.Call
}

// SynthTemplate is a helper method to define mock.On call
//   - app awscdk.App
func (_e *MockAWSInfraProviderer_Expecter) SynthTemplate(app interface{}) *MockAWSInfraProviderer_SynthTemplate_Call {
	return &MockAWSInfraProviderer_SynthTemplate_Call{Call: _e.mock.On("SynthTemplate", app)}
}

func (_c *MockAWSInfraProviderer_SynthTemplate_Call) Run(run func(app awscdk.App)) *MockAWSInfraProviderer_SynthTemplate_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(awscdk.App))
	})
	return _c
}

func (_c *MockAWSInfraProviderer_SynthTemplate_Call) Return(_a0 string, _a1 error) *MockAWSInfraProviderer_SynthTemplate_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockAWSInfraProviderer_SynthTemplate_Call) RunAndReturn(run func(awscdk.App) (string, error)) *MockAWSInfraProviderer_SynthTemplate_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockAWSInfraProviderer creates a new instance of MockAWSInfraProviderer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockAWSInfraProviderer(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockAWSInfraProviderer {
	mock := &MockAWSInfraProviderer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

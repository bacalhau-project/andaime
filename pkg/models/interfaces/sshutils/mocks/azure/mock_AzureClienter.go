// Code generated by mockery v2.46.3. DO NOT EDIT.

package azure_interface

import (
	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	armnetwork "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"

	armresources "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"

	armsubscription "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"

	azure_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/azure"

	context "context"

	mock "github.com/stretchr/testify/mock"

	runtime "github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
)

// MockAzureClienter is an autogenerated mock type for the AzureClienter type
type MockAzureClienter struct {
	mock.Mock
}

type MockAzureClienter_Expecter struct {
	mock *mock.Mock
}

func (_m *MockAzureClienter) EXPECT() *MockAzureClienter_Expecter {
	return &MockAzureClienter_Expecter{mock: &_m.Mock}
}

// DeployTemplate provides a mock function with given fields: ctx, resourceGroupName, deploymentName, template, params, tags
func (_m *MockAzureClienter) DeployTemplate(ctx context.Context, resourceGroupName string, deploymentName string, template map[string]interface{}, params map[string]interface{}, tags map[string]*string) (azure_interface.Pollerer, error) {
	ret := _m.Called(ctx, resourceGroupName, deploymentName, template, params, tags)

	if len(ret) == 0 {
		panic("no return value specified for DeployTemplate")
	}

	var r0 azure_interface.Pollerer
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, map[string]interface{}, map[string]interface{}, map[string]*string) (azure_interface.Pollerer, error)); ok {
		return rf(ctx, resourceGroupName, deploymentName, template, params, tags)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string, map[string]interface{}, map[string]interface{}, map[string]*string) azure_interface.Pollerer); ok {
		r0 = rf(ctx, resourceGroupName, deploymentName, template, params, tags)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(azure_interface.Pollerer)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string, map[string]interface{}, map[string]interface{}, map[string]*string) error); ok {
		r1 = rf(ctx, resourceGroupName, deploymentName, template, params, tags)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockAzureClienter_DeployTemplate_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DeployTemplate'
type MockAzureClienter_DeployTemplate_Call struct {
	*mock.Call
}

// DeployTemplate is a helper method to define mock.On call
//   - ctx context.Context
//   - resourceGroupName string
//   - deploymentName string
//   - template map[string]interface{}
//   - params map[string]interface{}
//   - tags map[string]*string
func (_e *MockAzureClienter_Expecter) DeployTemplate(ctx interface{}, resourceGroupName interface{}, deploymentName interface{}, template interface{}, params interface{}, tags interface{}) *MockAzureClienter_DeployTemplate_Call {
	return &MockAzureClienter_DeployTemplate_Call{Call: _e.mock.On("DeployTemplate", ctx, resourceGroupName, deploymentName, template, params, tags)}
}

func (_c *MockAzureClienter_DeployTemplate_Call) Run(run func(ctx context.Context, resourceGroupName string, deploymentName string, template map[string]interface{}, params map[string]interface{}, tags map[string]*string)) *MockAzureClienter_DeployTemplate_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string), args[3].(map[string]interface{}), args[4].(map[string]interface{}), args[5].(map[string]*string))
	})
	return _c
}

func (_c *MockAzureClienter_DeployTemplate_Call) Return(_a0 azure_interface.Pollerer, _a1 error) *MockAzureClienter_DeployTemplate_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockAzureClienter_DeployTemplate_Call) RunAndReturn(run func(context.Context, string, string, map[string]interface{}, map[string]interface{}, map[string]*string) (azure_interface.Pollerer, error)) *MockAzureClienter_DeployTemplate_Call {
	_c.Call.Return(run)
	return _c
}

// DestroyResourceGroup provides a mock function with given fields: ctx, resourceGroupName
func (_m *MockAzureClienter) DestroyResourceGroup(ctx context.Context, resourceGroupName string) error {
	ret := _m.Called(ctx, resourceGroupName)

	if len(ret) == 0 {
		panic("no return value specified for DestroyResourceGroup")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, resourceGroupName)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockAzureClienter_DestroyResourceGroup_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DestroyResourceGroup'
type MockAzureClienter_DestroyResourceGroup_Call struct {
	*mock.Call
}

// DestroyResourceGroup is a helper method to define mock.On call
//   - ctx context.Context
//   - resourceGroupName string
func (_e *MockAzureClienter_Expecter) DestroyResourceGroup(ctx interface{}, resourceGroupName interface{}) *MockAzureClienter_DestroyResourceGroup_Call {
	return &MockAzureClienter_DestroyResourceGroup_Call{Call: _e.mock.On("DestroyResourceGroup", ctx, resourceGroupName)}
}

func (_c *MockAzureClienter_DestroyResourceGroup_Call) Run(run func(ctx context.Context, resourceGroupName string)) *MockAzureClienter_DestroyResourceGroup_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockAzureClienter_DestroyResourceGroup_Call) Return(_a0 error) *MockAzureClienter_DestroyResourceGroup_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockAzureClienter_DestroyResourceGroup_Call) RunAndReturn(run func(context.Context, string) error) *MockAzureClienter_DestroyResourceGroup_Call {
	_c.Call.Return(run)
	return _c
}

// GetNetworkInterface provides a mock function with given fields: ctx, resourceGroupName, networkInterfaceName
func (_m *MockAzureClienter) GetNetworkInterface(ctx context.Context, resourceGroupName string, networkInterfaceName string) (*armnetwork.Interface, error) {
	ret := _m.Called(ctx, resourceGroupName, networkInterfaceName)

	if len(ret) == 0 {
		panic("no return value specified for GetNetworkInterface")
	}

	var r0 *armnetwork.Interface
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*armnetwork.Interface, error)); ok {
		return rf(ctx, resourceGroupName, networkInterfaceName)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *armnetwork.Interface); ok {
		r0 = rf(ctx, resourceGroupName, networkInterfaceName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*armnetwork.Interface)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, resourceGroupName, networkInterfaceName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockAzureClienter_GetNetworkInterface_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetNetworkInterface'
type MockAzureClienter_GetNetworkInterface_Call struct {
	*mock.Call
}

// GetNetworkInterface is a helper method to define mock.On call
//   - ctx context.Context
//   - resourceGroupName string
//   - networkInterfaceName string
func (_e *MockAzureClienter_Expecter) GetNetworkInterface(ctx interface{}, resourceGroupName interface{}, networkInterfaceName interface{}) *MockAzureClienter_GetNetworkInterface_Call {
	return &MockAzureClienter_GetNetworkInterface_Call{Call: _e.mock.On("GetNetworkInterface", ctx, resourceGroupName, networkInterfaceName)}
}

func (_c *MockAzureClienter_GetNetworkInterface_Call) Run(run func(ctx context.Context, resourceGroupName string, networkInterfaceName string)) *MockAzureClienter_GetNetworkInterface_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string))
	})
	return _c
}

func (_c *MockAzureClienter_GetNetworkInterface_Call) Return(_a0 *armnetwork.Interface, _a1 error) *MockAzureClienter_GetNetworkInterface_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockAzureClienter_GetNetworkInterface_Call) RunAndReturn(run func(context.Context, string, string) (*armnetwork.Interface, error)) *MockAzureClienter_GetNetworkInterface_Call {
	_c.Call.Return(run)
	return _c
}

// GetOrCreateResourceGroup provides a mock function with given fields: ctx, resourceGroupName, location, tags
func (_m *MockAzureClienter) GetOrCreateResourceGroup(ctx context.Context, resourceGroupName string, location string, tags map[string]string) (*armresources.ResourceGroup, error) {
	ret := _m.Called(ctx, resourceGroupName, location, tags)

	if len(ret) == 0 {
		panic("no return value specified for GetOrCreateResourceGroup")
	}

	var r0 *armresources.ResourceGroup
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, map[string]string) (*armresources.ResourceGroup, error)); ok {
		return rf(ctx, resourceGroupName, location, tags)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string, map[string]string) *armresources.ResourceGroup); ok {
		r0 = rf(ctx, resourceGroupName, location, tags)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*armresources.ResourceGroup)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string, map[string]string) error); ok {
		r1 = rf(ctx, resourceGroupName, location, tags)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockAzureClienter_GetOrCreateResourceGroup_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetOrCreateResourceGroup'
type MockAzureClienter_GetOrCreateResourceGroup_Call struct {
	*mock.Call
}

// GetOrCreateResourceGroup is a helper method to define mock.On call
//   - ctx context.Context
//   - resourceGroupName string
//   - location string
//   - tags map[string]string
func (_e *MockAzureClienter_Expecter) GetOrCreateResourceGroup(ctx interface{}, resourceGroupName interface{}, location interface{}, tags interface{}) *MockAzureClienter_GetOrCreateResourceGroup_Call {
	return &MockAzureClienter_GetOrCreateResourceGroup_Call{Call: _e.mock.On("GetOrCreateResourceGroup", ctx, resourceGroupName, location, tags)}
}

func (_c *MockAzureClienter_GetOrCreateResourceGroup_Call) Run(run func(ctx context.Context, resourceGroupName string, location string, tags map[string]string)) *MockAzureClienter_GetOrCreateResourceGroup_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string), args[3].(map[string]string))
	})
	return _c
}

func (_c *MockAzureClienter_GetOrCreateResourceGroup_Call) Return(_a0 *armresources.ResourceGroup, _a1 error) *MockAzureClienter_GetOrCreateResourceGroup_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockAzureClienter_GetOrCreateResourceGroup_Call) RunAndReturn(run func(context.Context, string, string, map[string]string) (*armresources.ResourceGroup, error)) *MockAzureClienter_GetOrCreateResourceGroup_Call {
	_c.Call.Return(run)
	return _c
}

// GetPublicIPAddress provides a mock function with given fields: ctx, resourceGroupName, publicIPAddress
func (_m *MockAzureClienter) GetPublicIPAddress(ctx context.Context, resourceGroupName string, publicIPAddress *armnetwork.PublicIPAddress) (string, error) {
	ret := _m.Called(ctx, resourceGroupName, publicIPAddress)

	if len(ret) == 0 {
		panic("no return value specified for GetPublicIPAddress")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *armnetwork.PublicIPAddress) (string, error)); ok {
		return rf(ctx, resourceGroupName, publicIPAddress)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, *armnetwork.PublicIPAddress) string); ok {
		r0 = rf(ctx, resourceGroupName, publicIPAddress)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, *armnetwork.PublicIPAddress) error); ok {
		r1 = rf(ctx, resourceGroupName, publicIPAddress)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockAzureClienter_GetPublicIPAddress_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetPublicIPAddress'
type MockAzureClienter_GetPublicIPAddress_Call struct {
	*mock.Call
}

// GetPublicIPAddress is a helper method to define mock.On call
//   - ctx context.Context
//   - resourceGroupName string
//   - publicIPAddress *armnetwork.PublicIPAddress
func (_e *MockAzureClienter_Expecter) GetPublicIPAddress(ctx interface{}, resourceGroupName interface{}, publicIPAddress interface{}) *MockAzureClienter_GetPublicIPAddress_Call {
	return &MockAzureClienter_GetPublicIPAddress_Call{Call: _e.mock.On("GetPublicIPAddress", ctx, resourceGroupName, publicIPAddress)}
}

func (_c *MockAzureClienter_GetPublicIPAddress_Call) Run(run func(ctx context.Context, resourceGroupName string, publicIPAddress *armnetwork.PublicIPAddress)) *MockAzureClienter_GetPublicIPAddress_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(*armnetwork.PublicIPAddress))
	})
	return _c
}

func (_c *MockAzureClienter_GetPublicIPAddress_Call) Return(_a0 string, _a1 error) *MockAzureClienter_GetPublicIPAddress_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockAzureClienter_GetPublicIPAddress_Call) RunAndReturn(run func(context.Context, string, *armnetwork.PublicIPAddress) (string, error)) *MockAzureClienter_GetPublicIPAddress_Call {
	_c.Call.Return(run)
	return _c
}

// GetResources provides a mock function with given fields: ctx, subscriptionID, resourceGroupName, tags
func (_m *MockAzureClienter) GetResources(ctx context.Context, subscriptionID string, resourceGroupName string, tags map[string]*string) ([]interface{}, error) {
	ret := _m.Called(ctx, subscriptionID, resourceGroupName, tags)

	if len(ret) == 0 {
		panic("no return value specified for GetResources")
	}

	var r0 []interface{}
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, map[string]*string) ([]interface{}, error)); ok {
		return rf(ctx, subscriptionID, resourceGroupName, tags)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string, map[string]*string) []interface{}); ok {
		r0 = rf(ctx, subscriptionID, resourceGroupName, tags)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]interface{})
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string, map[string]*string) error); ok {
		r1 = rf(ctx, subscriptionID, resourceGroupName, tags)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockAzureClienter_GetResources_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetResources'
type MockAzureClienter_GetResources_Call struct {
	*mock.Call
}

// GetResources is a helper method to define mock.On call
//   - ctx context.Context
//   - subscriptionID string
//   - resourceGroupName string
//   - tags map[string]*string
func (_e *MockAzureClienter_Expecter) GetResources(ctx interface{}, subscriptionID interface{}, resourceGroupName interface{}, tags interface{}) *MockAzureClienter_GetResources_Call {
	return &MockAzureClienter_GetResources_Call{Call: _e.mock.On("GetResources", ctx, subscriptionID, resourceGroupName, tags)}
}

func (_c *MockAzureClienter_GetResources_Call) Run(run func(ctx context.Context, subscriptionID string, resourceGroupName string, tags map[string]*string)) *MockAzureClienter_GetResources_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string), args[3].(map[string]*string))
	})
	return _c
}

func (_c *MockAzureClienter_GetResources_Call) Return(_a0 []interface{}, _a1 error) *MockAzureClienter_GetResources_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockAzureClienter_GetResources_Call) RunAndReturn(run func(context.Context, string, string, map[string]*string) ([]interface{}, error)) *MockAzureClienter_GetResources_Call {
	_c.Call.Return(run)
	return _c
}

// GetSKUsByLocation provides a mock function with given fields: ctx, location
func (_m *MockAzureClienter) GetSKUsByLocation(ctx context.Context, location string) ([]armcompute.ResourceSKU, error) {
	ret := _m.Called(ctx, location)

	if len(ret) == 0 {
		panic("no return value specified for GetSKUsByLocation")
	}

	var r0 []armcompute.ResourceSKU
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) ([]armcompute.ResourceSKU, error)); ok {
		return rf(ctx, location)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) []armcompute.ResourceSKU); ok {
		r0 = rf(ctx, location)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]armcompute.ResourceSKU)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, location)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockAzureClienter_GetSKUsByLocation_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetSKUsByLocation'
type MockAzureClienter_GetSKUsByLocation_Call struct {
	*mock.Call
}

// GetSKUsByLocation is a helper method to define mock.On call
//   - ctx context.Context
//   - location string
func (_e *MockAzureClienter_Expecter) GetSKUsByLocation(ctx interface{}, location interface{}) *MockAzureClienter_GetSKUsByLocation_Call {
	return &MockAzureClienter_GetSKUsByLocation_Call{Call: _e.mock.On("GetSKUsByLocation", ctx, location)}
}

func (_c *MockAzureClienter_GetSKUsByLocation_Call) Run(run func(ctx context.Context, location string)) *MockAzureClienter_GetSKUsByLocation_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockAzureClienter_GetSKUsByLocation_Call) Return(_a0 []armcompute.ResourceSKU, _a1 error) *MockAzureClienter_GetSKUsByLocation_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockAzureClienter_GetSKUsByLocation_Call) RunAndReturn(run func(context.Context, string) ([]armcompute.ResourceSKU, error)) *MockAzureClienter_GetSKUsByLocation_Call {
	_c.Call.Return(run)
	return _c
}

// GetVMExternalIP provides a mock function with given fields: ctx, vmName, locationData
func (_m *MockAzureClienter) GetVMExternalIP(ctx context.Context, vmName string, locationData map[string]string) (string, error) {
	ret := _m.Called(ctx, vmName, locationData)

	if len(ret) == 0 {
		panic("no return value specified for GetVMExternalIP")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, map[string]string) (string, error)); ok {
		return rf(ctx, vmName, locationData)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, map[string]string) string); ok {
		r0 = rf(ctx, vmName, locationData)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, map[string]string) error); ok {
		r1 = rf(ctx, vmName, locationData)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockAzureClienter_GetVMExternalIP_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetVMExternalIP'
type MockAzureClienter_GetVMExternalIP_Call struct {
	*mock.Call
}

// GetVMExternalIP is a helper method to define mock.On call
//   - ctx context.Context
//   - vmName string
//   - locationData map[string]string
func (_e *MockAzureClienter_Expecter) GetVMExternalIP(ctx interface{}, vmName interface{}, locationData interface{}) *MockAzureClienter_GetVMExternalIP_Call {
	return &MockAzureClienter_GetVMExternalIP_Call{Call: _e.mock.On("GetVMExternalIP", ctx, vmName, locationData)}
}

func (_c *MockAzureClienter_GetVMExternalIP_Call) Run(run func(ctx context.Context, vmName string, locationData map[string]string)) *MockAzureClienter_GetVMExternalIP_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(map[string]string))
	})
	return _c
}

func (_c *MockAzureClienter_GetVMExternalIP_Call) Return(_a0 string, _a1 error) *MockAzureClienter_GetVMExternalIP_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockAzureClienter_GetVMExternalIP_Call) RunAndReturn(run func(context.Context, string, map[string]string) (string, error)) *MockAzureClienter_GetVMExternalIP_Call {
	_c.Call.Return(run)
	return _c
}

// GetVirtualMachine provides a mock function with given fields: ctx, resourceGroupName, vmName
func (_m *MockAzureClienter) GetVirtualMachine(ctx context.Context, resourceGroupName string, vmName string) (*armcompute.VirtualMachine, error) {
	ret := _m.Called(ctx, resourceGroupName, vmName)

	if len(ret) == 0 {
		panic("no return value specified for GetVirtualMachine")
	}

	var r0 *armcompute.VirtualMachine
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*armcompute.VirtualMachine, error)); ok {
		return rf(ctx, resourceGroupName, vmName)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *armcompute.VirtualMachine); ok {
		r0 = rf(ctx, resourceGroupName, vmName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*armcompute.VirtualMachine)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, resourceGroupName, vmName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockAzureClienter_GetVirtualMachine_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetVirtualMachine'
type MockAzureClienter_GetVirtualMachine_Call struct {
	*mock.Call
}

// GetVirtualMachine is a helper method to define mock.On call
//   - ctx context.Context
//   - resourceGroupName string
//   - vmName string
func (_e *MockAzureClienter_Expecter) GetVirtualMachine(ctx interface{}, resourceGroupName interface{}, vmName interface{}) *MockAzureClienter_GetVirtualMachine_Call {
	return &MockAzureClienter_GetVirtualMachine_Call{Call: _e.mock.On("GetVirtualMachine", ctx, resourceGroupName, vmName)}
}

func (_c *MockAzureClienter_GetVirtualMachine_Call) Run(run func(ctx context.Context, resourceGroupName string, vmName string)) *MockAzureClienter_GetVirtualMachine_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string))
	})
	return _c
}

func (_c *MockAzureClienter_GetVirtualMachine_Call) Return(_a0 *armcompute.VirtualMachine, _a1 error) *MockAzureClienter_GetVirtualMachine_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockAzureClienter_GetVirtualMachine_Call) RunAndReturn(run func(context.Context, string, string) (*armcompute.VirtualMachine, error)) *MockAzureClienter_GetVirtualMachine_Call {
	_c.Call.Return(run)
	return _c
}

// ListAllResourceGroups provides a mock function with given fields: ctx
func (_m *MockAzureClienter) ListAllResourceGroups(ctx context.Context) ([]*armresources.ResourceGroup, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for ListAllResourceGroups")
	}

	var r0 []*armresources.ResourceGroup
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) ([]*armresources.ResourceGroup, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) []*armresources.ResourceGroup); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*armresources.ResourceGroup)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockAzureClienter_ListAllResourceGroups_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ListAllResourceGroups'
type MockAzureClienter_ListAllResourceGroups_Call struct {
	*mock.Call
}

// ListAllResourceGroups is a helper method to define mock.On call
//   - ctx context.Context
func (_e *MockAzureClienter_Expecter) ListAllResourceGroups(ctx interface{}) *MockAzureClienter_ListAllResourceGroups_Call {
	return &MockAzureClienter_ListAllResourceGroups_Call{Call: _e.mock.On("ListAllResourceGroups", ctx)}
}

func (_c *MockAzureClienter_ListAllResourceGroups_Call) Run(run func(ctx context.Context)) *MockAzureClienter_ListAllResourceGroups_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockAzureClienter_ListAllResourceGroups_Call) Return(_a0 []*armresources.ResourceGroup, _a1 error) *MockAzureClienter_ListAllResourceGroups_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockAzureClienter_ListAllResourceGroups_Call) RunAndReturn(run func(context.Context) ([]*armresources.ResourceGroup, error)) *MockAzureClienter_ListAllResourceGroups_Call {
	_c.Call.Return(run)
	return _c
}

// ListAllResourcesInSubscription provides a mock function with given fields: ctx, subscriptionID, tags
func (_m *MockAzureClienter) ListAllResourcesInSubscription(ctx context.Context, subscriptionID string, tags map[string]*string) ([]interface{}, error) {
	ret := _m.Called(ctx, subscriptionID, tags)

	if len(ret) == 0 {
		panic("no return value specified for ListAllResourcesInSubscription")
	}

	var r0 []interface{}
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, map[string]*string) ([]interface{}, error)); ok {
		return rf(ctx, subscriptionID, tags)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, map[string]*string) []interface{}); ok {
		r0 = rf(ctx, subscriptionID, tags)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]interface{})
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, map[string]*string) error); ok {
		r1 = rf(ctx, subscriptionID, tags)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockAzureClienter_ListAllResourcesInSubscription_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ListAllResourcesInSubscription'
type MockAzureClienter_ListAllResourcesInSubscription_Call struct {
	*mock.Call
}

// ListAllResourcesInSubscription is a helper method to define mock.On call
//   - ctx context.Context
//   - subscriptionID string
//   - tags map[string]*string
func (_e *MockAzureClienter_Expecter) ListAllResourcesInSubscription(ctx interface{}, subscriptionID interface{}, tags interface{}) *MockAzureClienter_ListAllResourcesInSubscription_Call {
	return &MockAzureClienter_ListAllResourcesInSubscription_Call{Call: _e.mock.On("ListAllResourcesInSubscription", ctx, subscriptionID, tags)}
}

func (_c *MockAzureClienter_ListAllResourcesInSubscription_Call) Run(run func(ctx context.Context, subscriptionID string, tags map[string]*string)) *MockAzureClienter_ListAllResourcesInSubscription_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(map[string]*string))
	})
	return _c
}

func (_c *MockAzureClienter_ListAllResourcesInSubscription_Call) Return(_a0 []interface{}, _a1 error) *MockAzureClienter_ListAllResourcesInSubscription_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockAzureClienter_ListAllResourcesInSubscription_Call) RunAndReturn(run func(context.Context, string, map[string]*string) ([]interface{}, error)) *MockAzureClienter_ListAllResourcesInSubscription_Call {
	_c.Call.Return(run)
	return _c
}

// NewSubscriptionListPager provides a mock function with given fields: ctx, options
func (_m *MockAzureClienter) NewSubscriptionListPager(ctx context.Context, options *armsubscription.SubscriptionsClientListOptions) *runtime.Pager[armsubscription.SubscriptionsClientListResponse] {
	ret := _m.Called(ctx, options)

	if len(ret) == 0 {
		panic("no return value specified for NewSubscriptionListPager")
	}

	var r0 *runtime.Pager[armsubscription.SubscriptionsClientListResponse]
	if rf, ok := ret.Get(0).(func(context.Context, *armsubscription.SubscriptionsClientListOptions) *runtime.Pager[armsubscription.SubscriptionsClientListResponse]); ok {
		r0 = rf(ctx, options)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*runtime.Pager[armsubscription.SubscriptionsClientListResponse])
		}
	}

	return r0
}

// MockAzureClienter_NewSubscriptionListPager_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'NewSubscriptionListPager'
type MockAzureClienter_NewSubscriptionListPager_Call struct {
	*mock.Call
}

// NewSubscriptionListPager is a helper method to define mock.On call
//   - ctx context.Context
//   - options *armsubscription.SubscriptionsClientListOptions
func (_e *MockAzureClienter_Expecter) NewSubscriptionListPager(ctx interface{}, options interface{}) *MockAzureClienter_NewSubscriptionListPager_Call {
	return &MockAzureClienter_NewSubscriptionListPager_Call{Call: _e.mock.On("NewSubscriptionListPager", ctx, options)}
}

func (_c *MockAzureClienter_NewSubscriptionListPager_Call) Run(run func(ctx context.Context, options *armsubscription.SubscriptionsClientListOptions)) *MockAzureClienter_NewSubscriptionListPager_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*armsubscription.SubscriptionsClientListOptions))
	})
	return _c
}

func (_c *MockAzureClienter_NewSubscriptionListPager_Call) Return(_a0 *runtime.Pager[armsubscription.SubscriptionsClientListResponse]) *MockAzureClienter_NewSubscriptionListPager_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockAzureClienter_NewSubscriptionListPager_Call) RunAndReturn(run func(context.Context, *armsubscription.SubscriptionsClientListOptions) *runtime.Pager[armsubscription.SubscriptionsClientListResponse]) *MockAzureClienter_NewSubscriptionListPager_Call {
	_c.Call.Return(run)
	return _c
}

// ResourceGroupExists provides a mock function with given fields: ctx, resourceGroupName
func (_m *MockAzureClienter) ResourceGroupExists(ctx context.Context, resourceGroupName string) (bool, error) {
	ret := _m.Called(ctx, resourceGroupName)

	if len(ret) == 0 {
		panic("no return value specified for ResourceGroupExists")
	}

	var r0 bool
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (bool, error)); ok {
		return rf(ctx, resourceGroupName)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) bool); ok {
		r0 = rf(ctx, resourceGroupName)
	} else {
		r0 = ret.Get(0).(bool)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, resourceGroupName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockAzureClienter_ResourceGroupExists_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ResourceGroupExists'
type MockAzureClienter_ResourceGroupExists_Call struct {
	*mock.Call
}

// ResourceGroupExists is a helper method to define mock.On call
//   - ctx context.Context
//   - resourceGroupName string
func (_e *MockAzureClienter_Expecter) ResourceGroupExists(ctx interface{}, resourceGroupName interface{}) *MockAzureClienter_ResourceGroupExists_Call {
	return &MockAzureClienter_ResourceGroupExists_Call{Call: _e.mock.On("ResourceGroupExists", ctx, resourceGroupName)}
}

func (_c *MockAzureClienter_ResourceGroupExists_Call) Run(run func(ctx context.Context, resourceGroupName string)) *MockAzureClienter_ResourceGroupExists_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockAzureClienter_ResourceGroupExists_Call) Return(_a0 bool, _a1 error) *MockAzureClienter_ResourceGroupExists_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockAzureClienter_ResourceGroupExists_Call) RunAndReturn(run func(context.Context, string) (bool, error)) *MockAzureClienter_ResourceGroupExists_Call {
	_c.Call.Return(run)
	return _c
}

// ValidateMachineType provides a mock function with given fields: ctx, location, vmSize
func (_m *MockAzureClienter) ValidateMachineType(ctx context.Context, location string, vmSize string) (bool, error) {
	ret := _m.Called(ctx, location, vmSize)

	if len(ret) == 0 {
		panic("no return value specified for ValidateMachineType")
	}

	var r0 bool
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (bool, error)); ok {
		return rf(ctx, location, vmSize)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) bool); ok {
		r0 = rf(ctx, location, vmSize)
	} else {
		r0 = ret.Get(0).(bool)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, location, vmSize)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockAzureClienter_ValidateMachineType_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ValidateMachineType'
type MockAzureClienter_ValidateMachineType_Call struct {
	*mock.Call
}

// ValidateMachineType is a helper method to define mock.On call
//   - ctx context.Context
//   - location string
//   - vmSize string
func (_e *MockAzureClienter_Expecter) ValidateMachineType(ctx interface{}, location interface{}, vmSize interface{}) *MockAzureClienter_ValidateMachineType_Call {
	return &MockAzureClienter_ValidateMachineType_Call{Call: _e.mock.On("ValidateMachineType", ctx, location, vmSize)}
}

func (_c *MockAzureClienter_ValidateMachineType_Call) Run(run func(ctx context.Context, location string, vmSize string)) *MockAzureClienter_ValidateMachineType_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string))
	})
	return _c
}

func (_c *MockAzureClienter_ValidateMachineType_Call) Return(_a0 bool, _a1 error) *MockAzureClienter_ValidateMachineType_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockAzureClienter_ValidateMachineType_Call) RunAndReturn(run func(context.Context, string, string) (bool, error)) *MockAzureClienter_ValidateMachineType_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockAzureClienter creates a new instance of MockAzureClienter. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockAzureClienter(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockAzureClienter {
	mock := &MockAzureClienter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

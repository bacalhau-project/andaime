// Code generated by mockery v2.38.0. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"

// MockClienter is an autogenerated mock type for the Clienter type
type MockClienter struct {
	mock.Mock
}

type MockClienter_Expecter struct {
	mock *mock.Mock
}

func (_m *MockClienter) EXPECT() *MockClienter_Expecter {
	return &MockClienter_Expecter{mock: &_m.Mock}
}

// NewMockClienter creates a new instance of MockClienter. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockClienter(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockClienter {
	mock := &MockClienter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

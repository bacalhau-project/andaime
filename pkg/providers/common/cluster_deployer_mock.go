package common

// type MockClusterDeployer struct {
// 	mock.Mock
// }

// func (cd *MockClusterDeployer) ProvisionMachines(ctx context.Context) error {
// 	return cd.Called(ctx).Error(0)
// }

// func (cd *MockClusterDeployer) FinalizeDeployment(ctx context.Context) error {
// 	return cd.Called(ctx).Error(0)
// }

// func (cd *MockClusterDeployer) ProvisionOrchestrator(
// 	ctx context.Context,
// 	machineName string,
// ) error {
// 	return cd.Called(ctx, machineName).Error(0)
// }

// func (cd *MockClusterDeployer) ProvisionWorker(
// 	ctx context.Context,
// 	machineName string,
// ) error {
// 	return cd.Called(ctx, machineName).Error(0)
// }

// func (cd *MockClusterDeployer) DeployWorker(ctx context.Context, machineName string) error {
// 	return cd.Called(ctx, machineName).Error(0)
// }

// func (cd *MockClusterDeployer) ProvisionPackagesOnMachine(
// 	ctx context.Context,
// 	machineName string,
// ) error {
// 	return cd.Called(ctx, machineName).Error(0)
// }

// func (cd *MockClusterDeployer) ProvisionSSH(ctx context.Context) error {
// 	return cd.Called(ctx).Error(0)
// }

// func (cd *MockClusterDeployer) WaitForAllMachinesToReachState(
// 	ctx context.Context,
// 	resourceType string,
// 	state models.MachineResourceState,
// ) error {
// 	return cd.Called(ctx, resourceType, state).Error(0)
// }

// func (cd *MockClusterDeployer) ProvisionBacalhauCluster(ctx context.Context) error {
// 	return cd.Called(ctx).Error(0)
// }

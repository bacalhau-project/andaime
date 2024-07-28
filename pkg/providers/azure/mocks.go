//nolint:lll
package azure

// type MockAzureClient struct {
// 	mock.Mock
// 	Logger                       *logger.Logger
// 	GetOrCreateResourceGroupFunc func(ctx context.Context, location string,
// 		name string,
// 		tags map[string]*string) (*armresources.ResourceGroup, error)
// 	DestroyResourceGroupFunc func(ctx context.Context, resourceGroupName string) error
// 	CreateVirtualNetworkFunc func(ctx context.Context,
// 		resourceGroupName,
// 		vnetName,
// 		location string,
// 		tags map[string]*string) (armnetwork.VirtualNetwork, error)
// 	GetVirtualNetworkFunc func(ctx context.Context, resourceGroupName, vnetName, location string) (armnetwork.VirtualNetwork, error)
// 	CreatePublicIPFunc    func(ctx context.Context, resourceGroupName,
// 		location,
// 		ipName string,
// 		tags map[string]*string) (armnetwork.PublicIPAddress, error)
// 	GetPublicIPFunc func(ctx context.Context,
// 		resourceGroupName,
// 		location,
// 		ipName string) (armnetwork.PublicIPAddress, error)
// 	CreateVirtualMachineFunc func(ctx context.Context,
// 		resourceGroupName,
// 		vmName string,
// 		parameters armcompute.VirtualMachine,
// 		tags map[string]*string) (armcompute.VirtualMachine, error)
// 	GetVirtualMachineFunc func(ctx context.Context,
// 		resourceGroupName,
// 		vmName string) (armcompute.VirtualMachine, error)
// 	CreateNetworkInterfaceFunc func(ctx context.Context,
// 		resourceGroupName,
// 		nicName string,
// 		parameters armnetwork.Interface,
// 		tags map[string]*string) (armnetwork.Interface, error)
// 	GetNetworkInterfaceFunc func(ctx context.Context,
// 		resourceGroupName,
// 		nicName string) (armnetwork.Interface, error)
// 	CreateNetworkSecurityGroupFunc func(ctx context.Context,
// 		deployment *models.Deployment,
// 		location string,
// 	) (armnetwork.SecurityGroup, error)
// 	GetNetworkSecurityGroupFunc func(ctx context.Context,
// 		resourceGroupName,
// 		sgName string) (armnetwork.SecurityGroup, error)
// 	SearchResourcesFunc func(ctx context.Context,
// 		resourceGroup string, subscriptionID string, tags map[string]*string) (armresourcegraph.ClientResourcesResponse, error)

// 	SubscriptionListPagerFunc func(ctx context.Context, options *armsubscription.SubscriptionsClientListOptions) *runtime.Pager[armsubscription.SubscriptionsClientListResponse]
// }

// func NewMockAzureClient() AzureClient {
// 	return &MockAzureClient{}
// }

// func (m *MockAzureClient) GetLogger() *logger.Logger {
// 	return m.Logger
// }

// func (m *MockAzureClient) SetLogger(logger *logger.Logger) {
// 	m.Logger = logger
// }

// func (m *MockAzureClient) GetOrCreateResourceGroup(ctx context.Context,
// 	location string,
// 	name string,
// 	tags map[string]*string) (*armresources.ResourceGroup, error) {
// 	return m.GetOrCreateResourceGroupFunc(ctx, location, name, tags)
// }

// func (m *MockAzureClient) DestroyResourceGroup(
// 	ctx context.Context,
// 	resourceGroupName string,
// ) error {
// 	return m.DestroyResourceGroupFunc(ctx, resourceGroupName)
// }

// func (m *MockAzureClient) CreateVirtualNetwork(
// 	ctx context.Context,
// 	resourceGroupName, vnetName, location string,
// 	tags map[string]*string,
// ) (armnetwork.VirtualNetwork, error) {
// 	return m.CreateVirtualNetworkFunc(ctx, resourceGroupName, vnetName, location, tags)
// }

// func (m *MockAzureClient) GetVirtualNetwork(
// 	ctx context.Context,
// 	resourceGroupName, vnetName, location string,
// ) (armnetwork.VirtualNetwork, error) {
// 	return m.GetVirtualNetworkFunc(ctx, resourceGroupName, vnetName, location)
// }

// func (m *MockAzureClient) CreatePublicIP(ctx context.Context,
// 	resourceGroupName, location, ipName string,
// 	tags map[string]*string) (armnetwork.PublicIPAddress, error) {
// 	return m.CreatePublicIPFunc(ctx, resourceGroupName, location, ipName, tags)
// }

// func (m *MockAzureClient) GetPublicIP(ctx context.Context,
// 	resourceGroupName, location, ipName string) (armnetwork.PublicIPAddress, error) {
// 	return m.GetPublicIPFunc(ctx, resourceGroupName, location, ipName)
// }

// func (m *MockAzureClient) CreateVirtualMachine(ctx context.Context,
// 	resourceGroupName,
// 	vmName string,
// 	parameters armcompute.VirtualMachine,
// 	tags map[string]*string) (armcompute.VirtualMachine, error) {
// 	return m.CreateVirtualMachineFunc(ctx, resourceGroupName, vmName, parameters, tags)
// }

// func (m *MockAzureClient) GetVirtualMachine(ctx context.Context,
// 	resourceGroupName, vmName string) (armcompute.VirtualMachine, error) {
// 	return m.GetVirtualMachineFunc(ctx, resourceGroupName, vmName)
// }

// func (m *MockAzureClient) CreateNetworkInterface(ctx context.Context,
// 	resourceGroupName, nicName string,
// 	parameters armnetwork.Interface,
// 	tags map[string]*string) (armnetwork.Interface, error) {
// 	return m.CreateNetworkInterfaceFunc(ctx, resourceGroupName, nicName, parameters, tags)
// }

// func (m *MockAzureClient) GetNetworkInterface(ctx context.Context,
// 	resourceGroupName, nicName string) (armnetwork.Interface, error) {
// 	return m.GetNetworkInterfaceFunc(ctx, resourceGroupName, nicName)
// }

// func (m *MockAzureClient) CreateNetworkSecurityGroup(ctx context.Context,
// 	deployment *models.Deployment,
// 	location string) (armnetwork.SecurityGroup, error) {
// 	return m.CreateNetworkSecurityGroupFunc(ctx, deployment, location)
// }

// func (m *MockAzureClient) GetNetworkSecurityGroup(ctx context.Context,
// 	resourceGroupName, sgName string) (armnetwork.SecurityGroup, error) {
// 	return m.GetNetworkSecurityGroupFunc(ctx, resourceGroupName, sgName)
// }

// func (m *MockAzureClient) SearchResources(
// 	ctx context.Context,
// 	resourceGroup string,
// 	subscriptionID string,
// 	tags map[string]*string,
// ) (armresourcegraph.ClientResourcesResponse, error) {
// 	return m.SearchResourcesFunc(ctx, resourceGroup, subscriptionID, tags)
// }

// func (m *MockAzureClient) NewSubscriptionListPager(
// 	ctx context.Context,
// 	options *armsubscription.SubscriptionsClientListOptions,
// ) *runtime.Pager[armsubscription.SubscriptionsClientListResponse] {
// 	return m.SubscriptionListPagerFunc(ctx, options)
// }

// type MockAzureProvider struct {
// 	mock.Mock
// }

// func (m *MockAzureProvider) CreateDeployment(ctx context.Context) error {
// 	args := m.Called(ctx)
// 	return args.Error(0)
// }

// var GetMockAzureProviderFunc = GetMockAzureProvider

// func GetMockAzureProvider() (*MockAzureProvider, error) {
// 	return &MockAzureProvider{}, nil
// }

// func GetMockAzureClient() AzureClient {
// 	return &MockAzureClient{
// 		Logger: logger.Get(),
// 	}
// }

// var _ AzureClient = &MockAzureClient{}

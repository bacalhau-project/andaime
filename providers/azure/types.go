package azure

import (
	"context"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
)

type ClientInterfaces struct {
	VirtualNetworksClient   VirtualNetworksClientAPI
	PublicIPAddressesClient PublicIPAddressesClientAPI
	NetworkInterfacesClient InterfacesClientAPI
	VirtualMachinesClient   VirtualMachinesClientAPI
	SecurityGroupsClient    SecurityGroupsClientAPI
	SSHWaiter               SSHWaiter
}

type SSHWaiter interface {
	WaitForSSH(publicIP, username string, privateKey []byte) error
}

type Poller[T any] interface {
	Done() bool
	Poll(context.Context) (*http.Response, error)
	Result(context.Context) (T, error)
	PollUntilDone(context.Context, *runtime.PollUntilDoneOptions) (T, error)
}

type MockPoller[T any] struct {
	result T
}

func (m *MockPoller[T]) Done() bool {
	return true
}

func (m *MockPoller[T]) Poll(ctx context.Context) (*http.Response, error) {
	return &http.Response{}, nil
}

func (m *MockPoller[T]) FinalResponse(ctx context.Context) (*http.Response, error) {
	return &http.Response{}, nil
}

func (m *MockPoller[T]) Result(ctx context.Context) (T, error) {
	return m.result, nil
}

func (m *MockPoller[T]) PollUntilDone(
	ctx context.Context,
	options *runtime.PollUntilDoneOptions,
) (T, error) {
	return m.result, nil
}

// PublicIPAddressesClientWrapper wraps the PublicIPAddressesClient
type PublicIPAddressesClientWrapper struct {
	client *armnetwork.PublicIPAddressesClient
}

// BeginCreateOrUpdate wraps the original BeginCreateOrUpdate method
func (w *PublicIPAddressesClientWrapper) BeginCreateOrUpdate(
	ctx context.Context,
	resourceGroupName string,
	publicIPAddressName string,
	parameters armnetwork.PublicIPAddress,
	options *armnetwork.PublicIPAddressesClientBeginCreateOrUpdateOptions,
) (Poller[armnetwork.PublicIPAddressesClientCreateOrUpdateResponse], error) {
	return w.client.BeginCreateOrUpdate(
		ctx,
		resourceGroupName,
		publicIPAddressName,
		parameters,
		options,
	)
}

// Get wraps the original Get method
func (w *PublicIPAddressesClientWrapper) Get(
	ctx context.Context,
	resourceGroupName string,
	publicIPAddressName string,
	options *armnetwork.PublicIPAddressesClientGetOptions,
) (armnetwork.PublicIPAddressesClientGetResponse, error) {
	return w.client.Get(ctx, resourceGroupName, publicIPAddressName, options)
}

type PublicIPAddressesClientAPI interface {
	BeginCreateOrUpdate(
		ctx context.Context,
		resourceGroupName string,
		publicIPAddressName string,
		parameters armnetwork.PublicIPAddress,
		options *armnetwork.PublicIPAddressesClientBeginCreateOrUpdateOptions,
	) (Poller[armnetwork.PublicIPAddressesClientCreateOrUpdateResponse], error)

	Get(
		ctx context.Context,
		resourceGroupName string,
		publicIPAddressName string,
		options *armnetwork.PublicIPAddressesClientGetOptions,
	) (armnetwork.PublicIPAddressesClientGetResponse, error)
}

// VirtualNetworksClientAPI defines the methods we need from VirtualNetworksClient
type VirtualNetworksClientAPI interface {
	BeginCreateOrUpdate(
		ctx context.Context,
		resourceGroupName string,
		virtualNetworkName string,
		parameters armnetwork.VirtualNetwork,
		options *armnetwork.VirtualNetworksClientBeginCreateOrUpdateOptions,
	) (Poller[armnetwork.VirtualNetworksClientCreateOrUpdateResponse], error)
	Get(
		ctx context.Context,
		resourceGroupName string,
		virtualNetworkName string,
		options *armnetwork.VirtualNetworksClientGetOptions,
	) (armnetwork.VirtualNetworksClientGetResponse, error)
}

// VirtualMachinesClientAPI defines the methods we need from VirtualMachinesClient
type VirtualMachinesClientAPI interface {
	BeginCreateOrUpdate(
		ctx context.Context,
		resourceGroupName string,
		vmName string,
		parameters armcompute.VirtualMachine,
		options *armcompute.VirtualMachinesClientBeginCreateOrUpdateOptions,
	) (Poller[armcompute.VirtualMachinesClientCreateOrUpdateResponse], error)
}
